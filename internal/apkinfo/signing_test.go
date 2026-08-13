package apkinfo

import (
	"archive/zip"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeLenPrefixed(w *bytes.Buffer, data []byte) {
	binary.Write(w, binary.LittleEndian, uint32(len(data)))
	w.Write(data)
}

// selfSignedCertDER generates a throwaway self-signed certificate purely
// for test fixtures - never used for anything security-relevant.
func selfSignedCertDER(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(12345),
		Subject:      pkix.Name{CommonName: "Test Signer"},
		NotBefore:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:     time.Date(2050, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

// buildSigningBlockValue builds a v2/v3 signature-scheme block's "value"
// bytes containing exactly one signer with exactly one certificate - the
// minimum extractCertificates actually reads (it never looks at
// signatures/public-key, so this test fixture omits them).
func buildSigningBlockValue(certDER []byte) []byte {
	var certsSeq bytes.Buffer
	writeLenPrefixed(&certsSeq, certDER)

	var signedData bytes.Buffer
	writeLenPrefixed(&signedData, nil)              // digests sequence: empty
	writeLenPrefixed(&signedData, certsSeq.Bytes()) // certificates sequence: one cert

	var signer bytes.Buffer
	writeLenPrefixed(&signer, signedData.Bytes())

	var signerSeq bytes.Buffer
	writeLenPrefixed(&signerSeq, signer.Bytes())

	var value bytes.Buffer
	writeLenPrefixed(&value, signerSeq.Bytes())
	return value.Bytes()
}

// buildSigningBlock assembles a full APK Signing Block containing one
// ID-value pair (schemeID -> value), per the v2 block format: size1,
// pairs, size2 (repeated), 16-byte magic.
func buildSigningBlock(schemeID uint32, value []byte) []byte {
	var pair bytes.Buffer
	binary.Write(&pair, binary.LittleEndian, schemeID)
	pair.Write(value)

	var pairs bytes.Buffer
	binary.Write(&pairs, binary.LittleEndian, uint64(pair.Len())) // length of (ID + value)
	pairs.Write(pair.Bytes())

	blockContentLen := pairs.Len() + 8 + 16
	var block bytes.Buffer
	binary.Write(&block, binary.LittleEndian, uint64(blockContentLen))
	block.Write(pairs.Bytes())
	binary.Write(&block, binary.LittleEndian, uint64(blockContentLen))
	block.WriteString(apkSigBlockMagic)
	return block.Bytes()
}

func TestParseSigningBlockExtractsV2Certificate(t *testing.T) {
	certDER := selfSignedCertDER(t)
	block := buildSigningBlock(idSignatureV2, buildSigningBlockValue(certDER))

	certs, hasV2, hasV3, err := parseSigningBlock(block)
	if err != nil {
		t.Fatalf("parseSigningBlock: %v", err)
	}
	if !hasV2 || hasV3 {
		t.Fatalf("expected hasV2=true hasV3=false, got hasV2=%v hasV3=%v", hasV2, hasV3)
	}
	if len(certs) != 1 {
		t.Fatalf("expected exactly 1 certificate, got %d", len(certs))
	}
	if certs[0].SerialNumber != "12345" {
		t.Errorf("SerialNumber = %q, want 12345", certs[0].SerialNumber)
	}
	if certs[0].Subject == "" || certs[0].SHA256 == "" {
		t.Errorf("expected Subject and SHA256 to be populated, got %+v", certs[0])
	}
}

func TestParseSigningBlockExtractsV3Certificate(t *testing.T) {
	certDER := selfSignedCertDER(t)
	block := buildSigningBlock(idSignatureV3, buildSigningBlockValue(certDER))

	_, hasV2, hasV3, err := parseSigningBlock(block)
	if err != nil {
		t.Fatalf("parseSigningBlock: %v", err)
	}
	if hasV2 || !hasV3 {
		t.Fatalf("expected hasV2=false hasV3=true, got hasV2=%v hasV3=%v", hasV2, hasV3)
	}
}

func TestParseSigningBlockIgnoresUnknownSchemeIDs(t *testing.T) {
	// A block containing only an unrecognized ID-value pair (e.g. a future
	// signature scheme, or the v1-padding entry some tools insert) must not
	// be mistaken for v2/v3, and must not error out either.
	block := buildSigningBlock(0xdeadbeef, []byte("irrelevant"))

	certs, hasV2, hasV3, err := parseSigningBlock(block)
	if err != nil {
		t.Fatalf("parseSigningBlock: %v", err)
	}
	if hasV2 || hasV3 || len(certs) != 0 {
		t.Fatalf("expected no scheme detected and no certs, got hasV2=%v hasV3=%v certs=%v", hasV2, hasV3, certs)
	}
}

// TestFindSigningBlockLocatesBlockBeforeCentralDirectory builds a
// synthetic byte layout matching the real one (arbitrary "zip entries" +
// signing block + arbitrary "central directory" + a minimal EOCD record
// pointing at it) and proves findSigningBlock recovers exactly the
// signing block bytes using only the EOCD's central-directory offset -
// the same backward-walk a real APK requires.
func TestFindSigningBlockLocatesBlockBeforeCentralDirectory(t *testing.T) {
	certDER := selfSignedCertDER(t)
	block := buildSigningBlock(idSignatureV2, buildSigningBlockValue(certDER))

	fakeEntries := []byte("pretend these are zip local file entries....")
	fakeCentralDir := []byte("pretend this is the zip central directory")

	var data bytes.Buffer
	data.Write(fakeEntries)
	cdOffset := data.Len()
	data.Write(block)
	data.Write(fakeCentralDir)

	// Minimal EOCD record (22 bytes, no comment): signature, 4x uint16
	// (disk numbers/counts - all 0/irrelevant here), cdSize (uint32),
	// cdOffset (uint32), commentLen (uint16, 0).
	var eocd bytes.Buffer
	binary.Write(&eocd, binary.LittleEndian, uint32(0x06054b50))
	binary.Write(&eocd, binary.LittleEndian, uint16(0)) // disk number
	binary.Write(&eocd, binary.LittleEndian, uint16(0)) // disk with CD start
	binary.Write(&eocd, binary.LittleEndian, uint16(1)) // entries on this disk
	binary.Write(&eocd, binary.LittleEndian, uint16(1)) // total entries
	binary.Write(&eocd, binary.LittleEndian, uint32(len(fakeCentralDir)))
	binary.Write(&eocd, binary.LittleEndian, uint32(cdOffset+len(block)))
	binary.Write(&eocd, binary.LittleEndian, uint16(0)) // comment length
	data.Write(eocd.Bytes())

	got, err := findSigningBlock(data.Bytes())
	if err != nil {
		t.Fatalf("findSigningBlock: %v", err)
	}
	if !bytes.Equal(got, block) {
		t.Fatalf("findSigningBlock returned %d bytes, want the original %d-byte block (mismatch)", len(got), len(block))
	}
}

func TestFindSigningBlockErrorsWhenAbsent(t *testing.T) {
	// A well-formed EOCD pointing at a central directory with no signing
	// block magic right before it - the common case for a v1-only-signed
	// or unsigned APK.
	fakeEntries := []byte("entries")
	fakeCentralDir := []byte("central directory, no signing block before it")

	var data bytes.Buffer
	data.Write(fakeEntries)
	cdOffset := data.Len()
	data.Write(fakeCentralDir)

	var eocd bytes.Buffer
	binary.Write(&eocd, binary.LittleEndian, uint32(0x06054b50))
	binary.Write(&eocd, binary.LittleEndian, uint16(0))
	binary.Write(&eocd, binary.LittleEndian, uint16(0))
	binary.Write(&eocd, binary.LittleEndian, uint16(1))
	binary.Write(&eocd, binary.LittleEndian, uint16(1))
	binary.Write(&eocd, binary.LittleEndian, uint32(len(fakeCentralDir)))
	binary.Write(&eocd, binary.LittleEndian, uint32(cdOffset))
	binary.Write(&eocd, binary.LittleEndian, uint16(0))
	data.Write(eocd.Bytes())

	if _, err := findSigningBlock(data.Bytes()); err == nil {
		t.Fatal("expected an error when no APK Signing Block magic is present")
	}
}

// TestAnalyzeSigningFallsBackToV1Detection proves a real (signing-block-
// free) zip containing a META-INF/*.RSA file is reported as v1-only,
// without a v2/v3 block or any decoded certificate.
func TestAnalyzeSigningFallsBackToV1Detection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.apk")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("META-INF/CERT.RSA")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("not a real PKCS7 blob, just needs to exist")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	info := AnalyzeSigning(&zr.Reader, path)
	if !info.SchemeV1Only {
		t.Error("expected SchemeV1Only=true for a zip with a META-INF/*.RSA file and no signing block")
	}
	if info.SchemeV2 || info.SchemeV3 || len(info.Certificates) != 0 {
		t.Errorf("expected no v2/v3 scheme or certificates, got %+v", info)
	}
}

func TestAnalyzeSigningNoSchemeDetectedForPlainZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plain.apk")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	if _, err := zw.Create("hello.txt"); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	info := AnalyzeSigning(&zr.Reader, path)
	if info.SchemeV1Only || info.SchemeV2 || info.SchemeV3 {
		t.Errorf("expected no signing scheme detected at all, got %+v", info)
	}
}
