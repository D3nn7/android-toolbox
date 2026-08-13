// Package apkinfo analyzes .apk files without shelling out to any external
// tool (no aapt, no Android SDK, no native binary) - everything here is
// pure Go, so it cross-compiles and runs identically on Windows, Linux and
// macOS. An .apk is a zip file whose AndroidManifest.xml is stored in
// Android's own binary XML format ("AXML") rather than plain text; this
// file implements just enough of that format (as documented by AOSP's
// frameworks/base ResourceTypes.h, and used the same way by tools like
// aapt/apktool/androguard) to walk the manifest as a small DOM.
//
// Deliberately NOT implemented: resolving @string/@drawable resource
// references against resources.arsc (that's a second, similarly-sized
// binary format) - a reference shows up as e.g. "@0x7f0e0012" rather than
// the resolved string. Most of what's useful for an "APK info" report
// (package name, version, SDK levels, permissions, activities) is given as
// literal values in the manifest and doesn't need that resolution at all.
package apkinfo

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"unicode/utf16"
)

// Chunk type constants (ResChunk_header.type), per AOSP ResourceTypes.h.
const (
	chunkTypeXML          = 0x0003
	chunkTypeStringPool   = 0x0001
	chunkTypeXMLResMap    = 0x0180
	chunkTypeStartNS      = 0x0100
	chunkTypeEndNS        = 0x0101
	chunkTypeStartElement = 0x0102
	chunkTypeEndElement   = 0x0103
	chunkTypeCData        = 0x0104
)

// ResValue.dataType constants, the subset relevant to manifest attributes.
const (
	typeReference  = 0x01
	typeString     = 0x03
	typeIntDec     = 0x10
	typeIntHex     = 0x11
	typeIntBoolean = 0x12
)

const stringPoolUTF8Flag = 0x00000100

// Node is one element of the parsed binary-XML tree (e.g. <manifest>,
// <uses-permission>, <activity>).
type Node struct {
	Name      string
	Namespace string
	Attrs     []Attr
	Children  []*Node
	Parent    *Node
}

// Attr is one already-resolved attribute value. Value is always a
// human-readable string; IntValue additionally carries the literal integer
// for attributes of an integer/hex/boolean type (versionCode,
// minSdkVersion, ...), since callers need the real number, not just its
// string form.
type Attr struct {
	Namespace string
	Name      string
	Value     string
	IntValue  int64
	Type      uint8
}

// Attr returns the named attribute (matched by local name only - manifest
// attributes all live in a single "android:" namespace in practice, so
// namespace-qualified lookups aren't needed here).
func (n *Node) Attr(name string) (Attr, bool) {
	for _, a := range n.Attrs {
		if a.Name == name {
			return a, true
		}
	}
	return Attr{}, false
}

// Child returns the first direct child element with the given name, or nil.
func (n *Node) Child(name string) *Node {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// ChildrenNamed returns every direct child element with the given name.
func (n *Node) ChildrenNamed(name string) []*Node {
	var out []*Node
	for _, c := range n.Children {
		if c.Name == name {
			out = append(out, c)
		}
	}
	return out
}

// ParseManifestXML parses an AndroidManifest.xml file's raw (binary XML)
// bytes into a Node tree rooted at the document element (normally
// <manifest>).
func ParseManifestXML(data []byte) (*Node, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("file is too small to be binary XML (%d bytes)", len(data))
	}
	if fileType := binary.LittleEndian.Uint16(data[0:]); fileType != chunkTypeXML {
		return nil, fmt.Errorf("unexpected root chunk type 0x%04x - not Android binary XML (plain-text AndroidManifest.xml isn't supported)", fileType)
	}

	pos := 8 // past the file-level ResChunk_header (type, headerSize, size)
	if pos+8 > len(data) || binary.LittleEndian.Uint16(data[pos:]) != chunkTypeStringPool {
		return nil, fmt.Errorf("expected a string pool chunk immediately after the XML header")
	}
	poolSize := int(binary.LittleEndian.Uint32(data[pos+4:]))
	if poolSize < 8 || pos+poolSize > len(data) {
		return nil, fmt.Errorf("string pool chunk has an invalid size")
	}
	strs, err := parseStringPoolChunk(data, pos)
	if err != nil {
		return nil, fmt.Errorf("string pool: %w", err)
	}
	pos += poolSize

	var root, current *Node
	var stack []*Node

	for pos+8 <= len(data) {
		chunkType := binary.LittleEndian.Uint16(data[pos:])
		chunkHeaderSize := int(binary.LittleEndian.Uint16(data[pos+2:]))
		chunkSize := int(binary.LittleEndian.Uint32(data[pos+4:]))
		if chunkSize < 8 || chunkHeaderSize < 8 || pos+chunkSize > len(data) {
			return nil, fmt.Errorf("malformed chunk at offset %d", pos)
		}

		switch chunkType {
		case chunkTypeStartElement:
			node, err := parseStartElement(data, pos, chunkHeaderSize, strs)
			if err != nil {
				return nil, err
			}
			if current != nil {
				node.Parent = current
				current.Children = append(current.Children, node)
			} else if root == nil {
				root = node
			}
			stack = append(stack, node)
			current = node

		case chunkTypeEndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("end-element with no matching start-element at offset %d", pos)
			}
			stack = stack[:len(stack)-1]
			if len(stack) > 0 {
				current = stack[len(stack)-1]
			} else {
				current = nil
			}

		case chunkTypeXMLResMap, chunkTypeStartNS, chunkTypeEndNS, chunkTypeCData:
			// Resource-ID map and namespace declarations aren't needed to
			// read manifest values by name; CDATA/text content isn't used
			// anywhere meaningful in a real manifest either.
		}

		pos += chunkSize
	}

	if root == nil {
		return nil, fmt.Errorf("no root element found in AndroidManifest.xml")
	}
	return root, nil
}

// parseStartElement reads one ResXMLTree_attrExt (a START_ELEMENT chunk's
// extended data, right after its common ResXMLTree_node header) into a Node.
func parseStartElement(data []byte, chunkStart, headerSize int, strs []string) (*Node, error) {
	extStart := chunkStart + headerSize
	if extStart+20 > len(data) {
		return nil, fmt.Errorf("start-element chunk truncated at offset %d", chunkStart)
	}
	nsIdx := int32(binary.LittleEndian.Uint32(data[extStart:]))
	nameIdx := int32(binary.LittleEndian.Uint32(data[extStart+4:]))
	attrStart := int(binary.LittleEndian.Uint16(data[extStart+8:]))
	attrSize := int(binary.LittleEndian.Uint16(data[extStart+10:]))
	attrCount := int(binary.LittleEndian.Uint16(data[extStart+12:]))

	if attrSize < 20 {
		return nil, fmt.Errorf("start-element at offset %d has an implausibly small attribute size (%d)", chunkStart, attrSize)
	}

	node := &Node{
		Name:      poolString(strs, nameIdx),
		Namespace: poolString(strs, nsIdx),
	}

	attrsBase := extStart + attrStart
	for i := 0; i < attrCount; i++ {
		off := attrsBase + i*attrSize
		if off+20 > len(data) {
			return nil, fmt.Errorf("attribute %d of element %q truncated at offset %d", i, node.Name, off)
		}
		aNsIdx := int32(binary.LittleEndian.Uint32(data[off:]))
		aNameIdx := int32(binary.LittleEndian.Uint32(data[off+4:]))
		aRawIdx := int32(binary.LittleEndian.Uint32(data[off+8:]))
		// ResValue at off+12: uint16 size, uint8 res0, uint8 dataType, uint32 data.
		dataType := data[off+15]
		value := binary.LittleEndian.Uint32(data[off+16:])

		attr := Attr{
			Namespace: poolString(strs, aNsIdx),
			Name:      poolString(strs, aNameIdx),
			Type:      dataType,
		}
		resolveAttrValue(&attr, strs, aRawIdx, value)
		node.Attrs = append(node.Attrs, attr)
	}
	return node, nil
}

// resolveAttrValue fills in attr.Value (and IntValue, for numeric types)
// from either the attribute's raw string form (present for most
// string-typed attributes) or its typed value, interpreted per attr.Type.
func resolveAttrValue(attr *Attr, strs []string, rawIdx int32, data uint32) {
	if rawIdx >= 0 {
		attr.Value = poolString(strs, rawIdx)
		return
	}
	switch attr.Type {
	case typeString:
		attr.Value = poolString(strs, int32(data))
	case typeIntDec:
		attr.IntValue = int64(int32(data))
		attr.Value = strconv.FormatInt(attr.IntValue, 10)
	case typeIntHex:
		attr.IntValue = int64(int32(data))
		attr.Value = fmt.Sprintf("0x%x", data)
	case typeIntBoolean:
		attr.IntValue = int64(data)
		if data != 0 {
			attr.Value = "true"
		} else {
			attr.Value = "false"
		}
	case typeReference:
		attr.IntValue = int64(data)
		attr.Value = fmt.Sprintf("@0x%08x", data)
	default:
		attr.IntValue = int64(data)
		attr.Value = fmt.Sprintf("0x%08x", data)
	}
}

func poolString(strs []string, idx int32) string {
	if idx < 0 || int(idx) >= len(strs) {
		return ""
	}
	return strs[idx]
}

// parseStringPoolChunk decodes a RES_STRING_POOL_TYPE chunk starting at the
// absolute offset chunkStart, returning every string it contains in order
// (so later code can look strings up by their pool index).
func parseStringPoolChunk(data []byte, chunkStart int) ([]string, error) {
	if chunkStart+28 > len(data) {
		return nil, fmt.Errorf("chunk truncated")
	}
	headerSize := int(binary.LittleEndian.Uint16(data[chunkStart+2:]))
	stringCount := int(binary.LittleEndian.Uint32(data[chunkStart+8:]))
	flags := binary.LittleEndian.Uint32(data[chunkStart+16:])
	stringsStart := int(binary.LittleEndian.Uint32(data[chunkStart+20:]))

	if headerSize < 28 {
		return nil, fmt.Errorf("implausible header size %d", headerSize)
	}
	indicesStart := chunkStart + headerSize
	if stringCount < 0 || indicesStart+stringCount*4 > len(data) {
		return nil, fmt.Errorf("string index table truncated (count=%d)", stringCount)
	}

	utf8 := flags&stringPoolUTF8Flag != 0
	dataStart := chunkStart + stringsStart
	if dataStart < 0 || dataStart > len(data) {
		return nil, fmt.Errorf("string data start out of range")
	}

	strs := make([]string, stringCount)
	for i := 0; i < stringCount; i++ {
		idx := int(binary.LittleEndian.Uint32(data[indicesStart+i*4:]))
		offset := dataStart + idx
		var s string
		var ok bool
		if utf8 {
			s, ok = readUTF8PoolString(data, offset)
		} else {
			s, ok = readUTF16PoolString(data, offset)
		}
		if !ok {
			return nil, fmt.Errorf("string pool entry %d at offset %d is out of bounds", i, offset)
		}
		strs[i] = s
	}
	return strs, nil
}

// decodeLen8 reads a UTF-8-string-pool length value: 1 byte if <= 0x7F,
// otherwise 2 bytes with the top bit of the first marking "there's more".
func decodeLen8(data []byte, pos int) (length, next int, ok bool) {
	if pos >= len(data) {
		return 0, pos, false
	}
	b := data[pos]
	if b&0x80 != 0 {
		if pos+1 >= len(data) {
			return 0, pos, false
		}
		return int(b&0x7F)<<8 | int(data[pos+1]), pos + 2, true
	}
	return int(b), pos + 1, true
}

// decodeLen16 is decodeLen8's UTF-16-string-pool counterpart: one uint16 if
// <= 0x7FFF, otherwise two with the top bit of the first marking overflow.
func decodeLen16(data []byte, pos int) (length, next int, ok bool) {
	if pos+2 > len(data) {
		return 0, pos, false
	}
	v := int(binary.LittleEndian.Uint16(data[pos:]))
	if v&0x8000 != 0 {
		if pos+4 > len(data) {
			return 0, pos, false
		}
		return (v&0x7FFF)<<16 | int(binary.LittleEndian.Uint16(data[pos+2:])), pos + 4, true
	}
	return v, pos + 2, true
}

func readUTF8PoolString(data []byte, offset int) (string, bool) {
	if offset < 0 {
		return "", false
	}
	_, next, ok := decodeLen8(data, offset) // UTF-16 length hint, not needed
	if !ok {
		return "", false
	}
	byteLen, next2, ok := decodeLen8(data, next)
	if !ok {
		return "", false
	}
	if byteLen < 0 || next2+byteLen > len(data) {
		return "", false
	}
	return string(data[next2 : next2+byteLen]), true
}

func readUTF16PoolString(data []byte, offset int) (string, bool) {
	if offset < 0 {
		return "", false
	}
	charLen, next, ok := decodeLen16(data, offset)
	if !ok {
		return "", false
	}
	byteLen := charLen * 2
	if charLen < 0 || next+byteLen > len(data) {
		return "", false
	}
	units := make([]uint16, charLen)
	for i := 0; i < charLen; i++ {
		units[i] = binary.LittleEndian.Uint16(data[next+i*2:])
	}
	return string(utf16.Decode(units)), true
}
