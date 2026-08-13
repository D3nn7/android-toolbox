package apkinfo

import (
	"bytes"
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

// The tests in this package can't be checked against a real-world APK (no
// Android SDK/aapt is available in this environment, and downloading an
// arbitrary APK isn't appropriate here either) - so instead, this file
// implements a small test-only AXML *encoder* that independently follows
// the same AOSP-documented format the decoder in axml.go does. Building a
// tree with it and confirming ParseManifestXML/ExtractManifestInfo recover
// exactly what was put in is the strongest verification available without
// a real fixture; it won't catch a shared misunderstanding of the format,
// but it does catch encoder/decoder logic bugs, which is what actually
// tends to go wrong in this kind of bit-twiddling code.

// testAttr is one attribute to encode - exactly one of StrValue or IsInt
// should be used.
type testAttr struct {
	Name     string
	StrValue string
	IntValue int32
	IsInt    bool
}

func strAttr(name, value string) testAttr { return testAttr{Name: name, StrValue: value} }
func intAttr(name string, value int32) testAttr {
	return testAttr{Name: name, IntValue: value, IsInt: true}
}

// testElem is one element to encode, recursively.
type testElem struct {
	Name     string
	Attrs    []testAttr
	Children []testElem
}

// axmlBuilder accumulates a deduplicated string pool as elements/attributes
// referencing it are built, then serializes everything (string pool + XML
// chunks) into a byte slice matching the format ParseManifestXML expects.
type axmlBuilder struct {
	strings []string
	index   map[string]int32
}

func newAXMLBuilder() *axmlBuilder {
	return &axmlBuilder{index: map[string]int32{}}
}

func (b *axmlBuilder) str(s string) int32 {
	if idx, ok := b.index[s]; ok {
		return idx
	}
	idx := int32(len(b.strings))
	b.strings = append(b.strings, s)
	b.index[s] = idx
	return idx
}

// build serializes root into a complete AndroidManifest.xml-style binary
// XML document.
func (b *axmlBuilder) build(root testElem) []byte {
	// Pre-pass: touch every string the tree references so the pool
	// contains them (order doesn't matter for correctness - lookups are by
	// index, not position).
	var collect func(e testElem)
	collect = func(e testElem) {
		b.str(e.Name)
		for _, a := range e.Attrs {
			b.str(a.Name)
			if !a.IsInt {
				b.str(a.StrValue)
			}
		}
		for _, c := range e.Children {
			collect(c)
		}
	}
	collect(root)

	var body bytes.Buffer
	var emit func(e testElem)
	emit = func(e testElem) {
		writeStartElement(&body, b.str(e.Name), e.Attrs, b)
		for _, c := range e.Children {
			emit(c)
		}
		writeEndElement(&body, b.str(e.Name))
	}
	emit(root)

	pool := encodeUTF16StringPool(b.strings)

	var out bytes.Buffer
	totalSize := 8 + len(pool) + body.Len()
	writeChunkHeader(&out, chunkTypeXML, 8, totalSize)
	out.Write(pool)
	out.Write(body.Bytes())
	return out.Bytes()
}

func writeChunkHeader(w *bytes.Buffer, chunkType uint16, headerSize uint16, size int) {
	binary.Write(w, binary.LittleEndian, chunkType)
	binary.Write(w, binary.LittleEndian, headerSize)
	binary.Write(w, binary.LittleEndian, uint32(size))
}

// encodeUTF16StringPool matches real aapt output (UTF-16, not the
// also-valid-but-less-common UTF-8 pool encoding) - see decodeLen16 in
// axml.go for the corresponding read side.
func encodeUTF16StringPool(strs []string) []byte {
	var data bytes.Buffer
	offsets := make([]uint32, len(strs))
	for i, s := range strs {
		offsets[i] = uint32(data.Len())
		units := utf16.Encode([]rune(s))
		// decodeLen16 only needs the 1-uint16 form for anything this test
		// package ever encodes (all well under 0x7FFF chars).
		binary.Write(&data, binary.LittleEndian, uint16(len(units)))
		for _, u := range units {
			binary.Write(&data, binary.LittleEndian, u)
		}
		binary.Write(&data, binary.LittleEndian, uint16(0)) // trailing NUL
	}

	const headerSize = 28
	totalSize := headerSize + len(strs)*4 + data.Len()

	var out bytes.Buffer
	writeChunkHeader(&out, chunkTypeStringPool, headerSize, totalSize)
	binary.Write(&out, binary.LittleEndian, uint32(len(strs)))              // stringCount
	binary.Write(&out, binary.LittleEndian, uint32(0))                      // styleCount
	binary.Write(&out, binary.LittleEndian, uint32(0))                      // flags (UTF-16, not sorted)
	binary.Write(&out, binary.LittleEndian, uint32(headerSize+len(strs)*4)) // stringsStart
	binary.Write(&out, binary.LittleEndian, uint32(0))                      // stylesStart (none)
	for _, off := range offsets {
		binary.Write(&out, binary.LittleEndian, off)
	}
	out.Write(data.Bytes())
	return out.Bytes()
}

func writeStartElement(w *bytes.Buffer, nameIdx int32, attrs []testAttr, b *axmlBuilder) {
	const nodeHeaderSize = 16 // ResChunk_header(8) + lineNumber(4) + comment(4)
	const extSize = 20        // ResXMLTree_attrExt fixed part
	const attrStructSize = 20
	size := nodeHeaderSize + extSize + len(attrs)*attrStructSize

	writeChunkHeader(w, chunkTypeStartElement, nodeHeaderSize, size)
	binary.Write(w, binary.LittleEndian, int32(0))  // lineNumber
	binary.Write(w, binary.LittleEndian, int32(-1)) // comment
	binary.Write(w, binary.LittleEndian, int32(-1)) // ns
	binary.Write(w, binary.LittleEndian, nameIdx)
	binary.Write(w, binary.LittleEndian, uint16(extSize))        // attributeStart
	binary.Write(w, binary.LittleEndian, uint16(attrStructSize)) // attributeSize
	binary.Write(w, binary.LittleEndian, uint16(len(attrs)))     // attributeCount
	binary.Write(w, binary.LittleEndian, uint16(0))              // idIndex
	binary.Write(w, binary.LittleEndian, uint16(0))              // classIndex
	binary.Write(w, binary.LittleEndian, uint16(0))              // styleIndex

	for _, a := range attrs {
		binary.Write(w, binary.LittleEndian, int32(-1)) // ns
		binary.Write(w, binary.LittleEndian, b.str(a.Name))
		if a.IsInt {
			binary.Write(w, binary.LittleEndian, int32(-1)) // rawValue: none, typed only
			binary.Write(w, binary.LittleEndian, uint16(8)) // ResValue.size
			w.WriteByte(0)                                  // res0
			w.WriteByte(typeIntDec)                         // dataType
			binary.Write(w, binary.LittleEndian, uint32(a.IntValue))
		} else {
			binary.Write(w, binary.LittleEndian, b.str(a.StrValue)) // rawValue present
			binary.Write(w, binary.LittleEndian, uint16(8))
			w.WriteByte(0)
			w.WriteByte(typeString)
			binary.Write(w, binary.LittleEndian, uint32(b.str(a.StrValue)))
		}
	}
}

func writeEndElement(w *bytes.Buffer, nameIdx int32) {
	const nodeHeaderSize = 16
	const extSize = 8 // ResXMLTree_endElementExt: ns(4) + name(4)
	writeChunkHeader(w, chunkTypeEndElement, nodeHeaderSize, nodeHeaderSize+extSize)
	binary.Write(w, binary.LittleEndian, int32(0))  // lineNumber
	binary.Write(w, binary.LittleEndian, int32(-1)) // comment
	binary.Write(w, binary.LittleEndian, int32(-1)) // ns
	binary.Write(w, binary.LittleEndian, nameIdx)
}

func sampleManifestTree() testElem {
	return testElem{
		Name: "manifest",
		Attrs: []testAttr{
			strAttr("package", "com.example.app"),
			intAttr("versionCode", 42),
			strAttr("versionName", "1.2.3"),
		},
		Children: []testElem{
			{
				Name: "uses-sdk",
				Attrs: []testAttr{
					intAttr("minSdkVersion", 21),
					intAttr("targetSdkVersion", 33),
				},
			},
			{
				Name:  "uses-permission",
				Attrs: []testAttr{strAttr("name", "android.permission.INTERNET")},
			},
			{
				Name:  "uses-permission",
				Attrs: []testAttr{strAttr("name", "android.permission.CAMERA")},
			},
			{
				Name:  "uses-feature",
				Attrs: []testAttr{strAttr("name", "android.hardware.camera")},
			},
			{
				Name:  "application",
				Attrs: []testAttr{strAttr("label", "Example App")},
				Children: []testElem{
					{
						Name:  "activity",
						Attrs: []testAttr{strAttr("name", ".MainActivity")},
						Children: []testElem{
							{
								Name: "intent-filter",
								Children: []testElem{
									{Name: "action", Attrs: []testAttr{strAttr("name", "android.intent.action.MAIN")}},
									{Name: "category", Attrs: []testAttr{strAttr("name", "android.intent.category.LAUNCHER")}},
								},
							},
						},
					},
					{
						Name:  "activity",
						Attrs: []testAttr{strAttr("name", ".SettingsActivity")},
					},
				},
			},
		},
	}
}

func TestParseManifestXMLRoundTrip(t *testing.T) {
	data := newAXMLBuilder().build(sampleManifestTree())

	root, err := ParseManifestXML(data)
	if err != nil {
		t.Fatalf("ParseManifestXML: %v", err)
	}
	if root.Name != "manifest" {
		t.Fatalf("expected root element 'manifest', got %q", root.Name)
	}
	if pkg, ok := root.Attr("package"); !ok || pkg.Value != "com.example.app" {
		t.Fatalf("expected package=com.example.app, got %+v (ok=%v)", pkg, ok)
	}
	if vc, ok := root.Attr("versionCode"); !ok || vc.IntValue != 42 {
		t.Fatalf("expected versionCode=42, got %+v (ok=%v)", vc, ok)
	}

	sdk := root.Child("uses-sdk")
	if sdk == nil {
		t.Fatal("expected a uses-sdk child")
	}
	if a, ok := sdk.Attr("minSdkVersion"); !ok || a.IntValue != 21 {
		t.Fatalf("expected minSdkVersion=21, got %+v (ok=%v)", a, ok)
	}

	perms := root.ChildrenNamed("uses-permission")
	if len(perms) != 2 {
		t.Fatalf("expected 2 uses-permission elements, got %d", len(perms))
	}

	app := root.Child("application")
	if app == nil {
		t.Fatal("expected an application child")
	}
	activities := app.ChildrenNamed("activity")
	if len(activities) != 2 {
		t.Fatalf("expected 2 activities, got %d", len(activities))
	}
}

func TestParseManifestXMLRejectsNonBinaryXML(t *testing.T) {
	if _, err := ParseManifestXML([]byte("<manifest></manifest>")); err == nil {
		t.Fatal("expected plain-text XML to be rejected (not binary AXML)")
	}
}

func TestParseManifestXMLRejectsTruncatedData(t *testing.T) {
	data := newAXMLBuilder().build(sampleManifestTree())
	if _, err := ParseManifestXML(data[:len(data)/2]); err == nil {
		t.Fatal("expected truncated binary XML to produce an error, not a partial/garbage tree")
	}
}
