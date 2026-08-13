package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/config"
)

// The AXML/zip builder below is a deliberately minimal, standalone fixture
// for this package's own tests - internal/apkinfo's own (more complete)
// test-only builder lives in its _test.go files and, like any Go test
// helper, isn't importable from another package. This one only needs to
// produce a single fixed manifest (package/version/permission), unlike
// apkinfo's general-purpose one.

const (
	axmlChunkTypeXML          = 0x0003
	axmlChunkTypeStringPool   = 0x0001
	axmlChunkTypeStartElement = 0x0102
	axmlChunkTypeEndElement   = 0x0103
	axmlTypeString            = 0x03
	axmlTypeIntDec            = 0x10
)

type axmlTestAttr struct {
	name     string
	strValue string
	intValue int32
	isInt    bool
}

type axmlTestElem struct {
	name     string
	attrs    []axmlTestAttr
	children []axmlTestElem
}

func axmlWriteChunkHeader(w *bytes.Buffer, chunkType, headerSize uint16, size int) {
	binary.Write(w, binary.LittleEndian, chunkType)
	binary.Write(w, binary.LittleEndian, headerSize)
	binary.Write(w, binary.LittleEndian, uint32(size))
}

// buildTestManifestBytes serializes tree into binary AXML, the same format
// AndroidManifest.xml uses inside a real .apk.
func buildTestManifestBytes(tree axmlTestElem) []byte {
	strs := []string{}
	index := map[string]int32{}
	str := func(s string) int32 {
		if i, ok := index[s]; ok {
			return i
		}
		i := int32(len(strs))
		strs = append(strs, s)
		index[s] = i
		return i
	}

	var collect func(e axmlTestElem)
	collect = func(e axmlTestElem) {
		str(e.name)
		for _, a := range e.attrs {
			str(a.name)
			if !a.isInt {
				str(a.strValue)
			}
		}
		for _, c := range e.children {
			collect(c)
		}
	}
	collect(tree)

	var body bytes.Buffer
	var emit func(e axmlTestElem)
	emit = func(e axmlTestElem) {
		const nodeHeaderSize = 16
		const extSize = 20
		const attrStructSize = 20
		size := nodeHeaderSize + extSize + len(e.attrs)*attrStructSize
		axmlWriteChunkHeader(&body, axmlChunkTypeStartElement, nodeHeaderSize, size)
		binary.Write(&body, binary.LittleEndian, int32(0))
		binary.Write(&body, binary.LittleEndian, int32(-1))
		binary.Write(&body, binary.LittleEndian, int32(-1))
		binary.Write(&body, binary.LittleEndian, str(e.name))
		binary.Write(&body, binary.LittleEndian, uint16(extSize))
		binary.Write(&body, binary.LittleEndian, uint16(attrStructSize))
		binary.Write(&body, binary.LittleEndian, uint16(len(e.attrs)))
		binary.Write(&body, binary.LittleEndian, uint16(0))
		binary.Write(&body, binary.LittleEndian, uint16(0))
		binary.Write(&body, binary.LittleEndian, uint16(0))
		for _, a := range e.attrs {
			binary.Write(&body, binary.LittleEndian, int32(-1))
			binary.Write(&body, binary.LittleEndian, str(a.name))
			if a.isInt {
				binary.Write(&body, binary.LittleEndian, int32(-1))
				binary.Write(&body, binary.LittleEndian, uint16(8))
				body.WriteByte(0)
				body.WriteByte(axmlTypeIntDec)
				binary.Write(&body, binary.LittleEndian, uint32(a.intValue))
			} else {
				binary.Write(&body, binary.LittleEndian, str(a.strValue))
				binary.Write(&body, binary.LittleEndian, uint16(8))
				body.WriteByte(0)
				body.WriteByte(axmlTypeString)
				binary.Write(&body, binary.LittleEndian, uint32(str(a.strValue)))
			}
		}
		for _, c := range e.children {
			emit(c)
		}
		axmlWriteChunkHeader(&body, axmlChunkTypeEndElement, nodeHeaderSize, nodeHeaderSize+8)
		binary.Write(&body, binary.LittleEndian, int32(0))
		binary.Write(&body, binary.LittleEndian, int32(-1))
		binary.Write(&body, binary.LittleEndian, int32(-1))
		binary.Write(&body, binary.LittleEndian, str(e.name))
	}
	emit(tree)

	var data bytes.Buffer
	offsets := make([]uint32, len(strs))
	for i, s := range strs {
		offsets[i] = uint32(data.Len())
		units := utf16.Encode([]rune(s))
		binary.Write(&data, binary.LittleEndian, uint16(len(units)))
		for _, u := range units {
			binary.Write(&data, binary.LittleEndian, u)
		}
		binary.Write(&data, binary.LittleEndian, uint16(0))
	}
	const poolHeaderSize = 28
	poolTotal := poolHeaderSize + len(strs)*4 + data.Len()
	var pool bytes.Buffer
	axmlWriteChunkHeader(&pool, axmlChunkTypeStringPool, poolHeaderSize, poolTotal)
	binary.Write(&pool, binary.LittleEndian, uint32(len(strs)))
	binary.Write(&pool, binary.LittleEndian, uint32(0))
	binary.Write(&pool, binary.LittleEndian, uint32(0))
	binary.Write(&pool, binary.LittleEndian, uint32(poolHeaderSize+len(strs)*4))
	binary.Write(&pool, binary.LittleEndian, uint32(0))
	for _, off := range offsets {
		binary.Write(&pool, binary.LittleEndian, off)
	}
	pool.Write(data.Bytes())

	var out bytes.Buffer
	total := 8 + pool.Len() + body.Len()
	axmlWriteChunkHeader(&out, axmlChunkTypeXML, 8, total)
	out.Write(pool.Bytes())
	out.Write(body.Bytes())
	return out.Bytes()
}

func sampleManifestTreeForAppTest() axmlTestElem {
	return axmlTestElem{
		name: "manifest",
		attrs: []axmlTestAttr{
			{name: "package", strValue: "com.example.app"},
			{name: "versionCode", intValue: 42, isInt: true},
			{name: "versionName", strValue: "1.2.3"},
		},
	}
}

// buildTestAPKBytes wraps a manifest tree in a minimal, real zip archive -
// everything apkinfo.Analyze needs (AndroidManifest.xml as a valid zip
// entry) without any signing block.
func buildTestAPKBytes(t *testing.T, tree axmlTestElem) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("AndroidManifest.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(buildTestManifestBytes(tree)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func newTestAPKInfoModel(t *testing.T) Model {
	t.Helper()
	m := New(context.Background(), config.Paths{}, config.Settings{}, config.State{}, nil)
	m.width, m.height = 100, 30
	m.apkInfo = newAPKInfoScreen(m)
	m.current = screenAPKInfo
	return m
}

func TestNewAPKInfoScreenStartsInPickingStage(t *testing.T) {
	m := newTestAPKInfoModel(t)
	if m.apkInfo.stage != apkInfoPicking {
		t.Fatalf("expected a fresh APK Info screen to start in the picking stage, got %v", m.apkInfo.stage)
	}
	if len(m.apkInfo.picker.AllowedTypes) != 1 || m.apkInfo.picker.AllowedTypes[0] != ".apk" {
		t.Fatalf("expected the file picker to be restricted to .apk files, got %v", m.apkInfo.picker.AllowedTypes)
	}
}

func TestAPKInfoEscFromPickingReturnsToToolSelect(t *testing.T) {
	m := newTestAPKInfoModel(t)

	updated, _ := m.updateAPKInfo(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.current != screenToolSelect {
		t.Fatalf("expected esc while picking to return to the tool-select screen, current = %v", m.current)
	}
}

// TestAPKInfoPickerHeightSetAtConstruction proves the file picker is sized
// immediately from the model's already-known terminal size, rather than
// staying at its zero-value Height until some future live resize - a
// freshly-built filepicker.Model left at Height=0 collapses its pagination
// window to a single visible entry, which is exactly the "everything's
// squeezed onto one line" bug reported for this screen.
func TestAPKInfoPickerHeightSetAtConstruction(t *testing.T) {
	m := New(context.Background(), config.Paths{}, config.Settings{}, config.State{}, nil)
	m.width, m.height = 100, 30

	screen := newAPKInfoScreen(m)

	want := apkInfoPickerHeight(m.height)
	if want <= 0 {
		t.Fatalf("apkInfoPickerHeight(%d) = %d, want > 0", m.height, want)
	}
	if screen.picker.Height != want {
		t.Fatalf("expected a freshly built picker to have Height %d, got %d", want, screen.picker.Height)
	}
}

// TestAPKInfoEscWhileNestedGoesUpNotOut proves esc distinguishes "go up one
// directory" from "leave the tool": the filepicker's own default Back
// keybinding already includes esc, so this screen must only intercept it to
// leave once the user is back at the directory the tool was opened in -
// otherwise a user descending into a subfolder and pressing esc (the natural
// instinct) gets ejected from the tool entirely instead of navigating up.
func TestAPKInfoEscWhileNestedGoesUpNotOut(t *testing.T) {
	m := newTestAPKInfoModel(t)
	m.apkInfo.picker.CurrentDirectory = filepath.Join(m.apkInfo.startDir, "subfolder")

	updated, _ := m.updateAPKInfoPicking(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.current != screenAPKInfo {
		t.Fatalf("expected esc while nested in a subfolder to stay within the APK Info tool, current = %v", m.current)
	}
	if m.apkInfo.picker.CurrentDirectory != m.apkInfo.startDir {
		t.Fatalf("expected esc while nested to navigate back up to startDir %q, picker is at %q", m.apkInfo.startDir, m.apkInfo.picker.CurrentDirectory)
	}
}

// TestAPKInfoQuitsFromPicking proves "q" quits the app while browsing for a
// file - the footer has always advertised "[q] quit" here, but neither the
// filepicker component nor this screen's own esc-handling ever actually
// bound it, so pressing q silently did nothing.
func TestAPKInfoQuitsFromPicking(t *testing.T) {
	m := newTestAPKInfoModel(t)

	_, cmd := m.updateAPKInfoPicking(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected q while picking to return a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected a tea.QuitMsg, got %T", cmd())
	}
}

// TestAPKInfoQuitsFromResult is TestAPKInfoQuitsFromPicking's counterpart for
// the result stage, which had the same gap.
func TestAPKInfoQuitsFromResult(t *testing.T) {
	m := newTestAPKInfoModel(t)
	m.apkInfo.stage = apkInfoResult

	_, cmd := m.updateAPKInfoResult(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected q on the result stage to return a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected a tea.QuitMsg, got %T", cmd())
	}
}

// TestAPKInfoResultViewportSizedAtConstruction is apkInfoResultViewport's
// counterpart to TestAPKInfoPickerHeightSetAtConstruction: the result
// viewport must be usable immediately, not only after some future resize.
func TestAPKInfoResultViewportSizedAtConstruction(t *testing.T) {
	m := New(context.Background(), config.Paths{}, config.Settings{}, config.State{}, nil)
	m.width, m.height = 100, 30

	screen := newAPKInfoScreen(m)

	wantH := apkInfoResultViewportHeight(m.height)
	if wantH <= 0 {
		t.Fatalf("apkInfoResultViewportHeight(%d) = %d, want > 0", m.height, wantH)
	}
	if screen.viewport.Height != wantH {
		t.Fatalf("expected the result viewport to have Height %d, got %d", wantH, screen.viewport.Height)
	}
	if screen.viewport.Width != m.width {
		t.Fatalf("expected the result viewport to have Width %d, got %d", m.width, screen.viewport.Width)
	}
}

// TestAPKInfoResultScrollsPastVisibleHeight proves a report longer than the
// viewport's height (many permissions, in this case) can actually be
// scrolled - reproducing the original bug report, where a long report simply
// overflowed the terminal with no way to see the rest or even find the
// footer's key legend again.
func TestAPKInfoResultScrollsPastVisibleHeight(t *testing.T) {
	dir := t.TempDir()
	tree := sampleManifestTreeForAppTest()
	apkPath := filepath.Join(dir, "sample.apk")
	if err := os.WriteFile(apkPath, buildTestAPKBytes(t, tree), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(context.Background(), config.Paths{}, config.Settings{}, config.State{}, nil)
	// A short terminal so the manifest's permission list (added by
	// sampleManifestTreeForAppTest's caller below) overflows a handful of
	// visible rows without needing dozens of fixture permissions.
	m.width, m.height = 100, 12
	m.apkInfo = newAPKInfoScreen(m)
	m.current = screenAPKInfo

	selectAPKInDir(t, &m.apkInfo.picker, dir)
	updated, _ := m.updateAPKInfoPicking(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.apkInfo.stage != apkInfoResult {
		t.Fatalf("expected the result stage, got %v", m.apkInfo.stage)
	}
	totalLines := m.apkInfo.viewport.TotalLineCount()
	if totalLines <= m.apkInfo.viewport.Height {
		t.Fatalf("expected the report (%d lines) to exceed the viewport height (%d) for this test to be meaningful", totalLines, m.apkInfo.viewport.Height)
	}
	if m.apkInfo.viewport.AtBottom() {
		t.Fatalf("expected the viewport to start scrolled to the top, not the bottom")
	}

	updated, cmd := m.updateAPKInfoResult(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if cmd != nil {
		_ = cmd()
	}
	if m.apkInfo.viewport.YOffset == 0 {
		t.Fatalf("expected pressing down to scroll the report, but YOffset stayed at 0")
	}
}

// TestAPKInfoDidSelectFileTriggersAnalysis proves selecting a file actually
// runs apkinfo.Analyze and transitions to the result stage - via a real
// directory read (see selectAPKInDir) rather than faking picker state, so
// this exercises the actual filepicker.DidSelectFile detection path.
func TestAPKInfoDidSelectFileTriggersAnalysis(t *testing.T) {
	dir := t.TempDir()
	apkPath := filepath.Join(dir, "sample.apk")
	if err := os.WriteFile(apkPath, buildTestAPKBytes(t, sampleManifestTreeForAppTest()), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newTestAPKInfoModel(t)
	selectAPKInDir(t, &m.apkInfo.picker, dir)

	updated, _ := m.updateAPKInfoPicking(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.apkInfo.stage != apkInfoResult {
		t.Fatalf("expected selecting a file to move to the result stage, got %v", m.apkInfo.stage)
	}
	if m.apkInfo.resultErr != nil {
		t.Fatalf("expected a successful analysis, got error: %v", m.apkInfo.resultErr)
	}
	if m.apkInfo.result.Manifest.PackageName != "com.example.app" {
		t.Fatalf("expected the analyzed package name, got %q", m.apkInfo.result.Manifest.PackageName)
	}
}

// selectAPKInDir points picker at dir (which must contain exactly one file,
// sample.apk, and nothing else) and drives it through a real directory
// read, so its internal file list/selection state is genuine rather than
// faked - filepicker.Model.DidSelectFile checks the actual listed entry at
// the current cursor position, not any field a test could just set
// directly.
func selectAPKInDir(t *testing.T, picker *filepicker.Model, dir string) {
	t.Helper()
	picker.CurrentDirectory = dir
	cmd := picker.Init()
	if cmd == nil {
		t.Fatal("expected Init to return a readDir command")
	}
	msg := cmd()
	var updateCmd tea.Cmd
	*picker, updateCmd = picker.Update(msg)
	if updateCmd != nil {
		// A real run schedules a repeating "spinner" tick alongside the
		// one-shot readDirMsg; irrelevant here, but drain it so nothing is
		// left dangling.
		_ = updateCmd()
	}
}

func TestAPKInfoEscFromResultReturnsToPicking(t *testing.T) {
	m := newTestAPKInfoModel(t)
	m.apkInfo.stage = apkInfoResult

	updated, _ := m.updateAPKInfoResult(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.apkInfo.stage != apkInfoPicking {
		t.Fatalf("expected esc from the result stage to return to picking, got %v", m.apkInfo.stage)
	}
	if m.current != screenAPKInfo {
		t.Fatalf("expected esc from the result stage to stay within the APK Info tool, current = %v", m.current)
	}
}

func TestAPKInfoRenderResultIncludesKeyFields(t *testing.T) {
	dir := t.TempDir()
	apkPath := filepath.Join(dir, "sample.apk")
	if err := os.WriteFile(apkPath, buildTestAPKBytes(t, sampleManifestTreeForAppTest()), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newTestAPKInfoModel(t)
	selectAPKInDir(t, &m.apkInfo.picker, dir)
	updated, _ := m.updateAPKInfoPicking(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	out := m.viewAPKInfoResult()
	for _, want := range []string{"com.example.app", "1.2.3", "42"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected the result view to contain %q, got:\n%s", want, out)
		}
	}
}
