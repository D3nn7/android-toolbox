package apkinfo

import (
	"archive/zip"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// APK Signing Block v2/v3 constants, per developer.android.com/apk/apk-signing
// ("APK Signature Scheme v2/v3 Block Format"). The block embeds raw X.509
// DER certificates directly (unlike the older JAR/v1 scheme, which wraps
// them in a PKCS#7 SignedData structure) - crypto/x509 alone is enough to
// read them, no extra parsing library needed.
const (
	apkSigBlockMagic = "APK Sig Block 42"
	idSignatureV2    = 0x7109871a
	idSignatureV3    = 0xf05368c0
)

// CertInfo summarizes one signing certificate found in the APK.
type CertInfo struct {
	Subject      string `json:"subject"`
	Issuer       string `json:"issuer"`
	SerialNumber string `json:"serialNumber"`
	NotBefore    string `json:"notBefore"`
	NotAfter     string `json:"notAfter"`
	SHA256       string `json:"sha256"` // fingerprint of the DER-encoded certificate itself
}

// SigningInfo reports what could be determined about how the APK is
// signed.
type SigningInfo struct {
	SchemeV2 bool `json:"schemeV2"`
	SchemeV3 bool `json:"schemeV3"`
	// SchemeV1Only is set when no v2/v3 signing block was found but
	// META-INF contains a JAR (v1) signature file - certificate details
	// for v1-only signing aren't decoded (that would require parsing a
	// PKCS#7 SignedData structure, which the v2/v3 block sidesteps
	// entirely by embedding raw X.509 DER certificates instead).
	SchemeV1Only bool       `json:"schemeV1Only"`
	Certificates []CertInfo `json:"certificates,omitempty"`
	// Err is set when a signing block was found but couldn't be parsed
	// (a malformed/unexpected structure) - distinct from "no block at all".
	Err string `json:"error,omitempty"`
}

// AnalyzeSigning locates the APK Signing Block (v2/v3) if present and
// summarizes its signer certificate(s).
func AnalyzeSigning(zr *zip.Reader, path string) SigningInfo {
	var info SigningInfo

	data, err := os.ReadFile(path)
	if err != nil {
		info.Err = err.Error()
		return info
	}

	block, err := findSigningBlock(data)
	if err != nil {
		// Most commonly this just means there's no v2/v3 block - not
		// itself an error worth surfacing. Note v1-only signing if
		// META-INF suggests JAR signing is present instead.
		info.SchemeV1Only = hasV1SignatureFiles(zr)
		return info
	}

	certs, hasV2, hasV3, parseErr := parseSigningBlock(block)
	info.SchemeV2 = hasV2
	info.SchemeV3 = hasV3
	info.Certificates = certs
	if parseErr != nil {
		info.Err = parseErr.Error()
	}
	if !hasV2 && !hasV3 {
		info.SchemeV1Only = hasV1SignatureFiles(zr)
	}
	return info
}

func hasV1SignatureFiles(zr *zip.Reader) bool {
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "META-INF/") {
			continue
		}
		if strings.HasSuffix(f.Name, ".RSA") || strings.HasSuffix(f.Name, ".DSA") || strings.HasSuffix(f.Name, ".EC") {
			return true
		}
	}
	return false
}

// findSigningBlock locates the APK Signing Block by walking backward from
// the ZIP End Of Central Directory record: the block sits immediately
// before the central directory, ending in a fixed 16-byte magic string
// preceded by a repeated "size of block" field that lets a reader find its
// start without needing to scan forward from the beginning of the file.
func findSigningBlock(data []byte) ([]byte, error) {
	eocd, err := findEOCD(data)
	if err != nil {
		return nil, err
	}
	if eocd+20 > len(data) {
		return nil, fmt.Errorf("truncated End Of Central Directory record")
	}
	cdOffset := int(binary.LittleEndian.Uint32(data[eocd+16:]))

	if cdOffset < 24 || cdOffset > len(data) {
		return nil, fmt.Errorf("no room for a signing block before the central directory")
	}
	magicStart := cdOffset - 16
	if string(data[magicStart:cdOffset]) != apkSigBlockMagic {
		return nil, fmt.Errorf("no APK Signing Block found")
	}

	size2Off := magicStart - 8
	if size2Off < 0 {
		return nil, fmt.Errorf("truncated signing block size field")
	}
	size2 := binary.LittleEndian.Uint64(data[size2Off:])
	blockStart := cdOffset - 8 - int(size2)
	if blockStart < 0 || blockStart+8 > len(data) {
		return nil, fmt.Errorf("signing block size field is out of range")
	}
	size1 := binary.LittleEndian.Uint64(data[blockStart:])
	if size1 != size2 {
		return nil, fmt.Errorf("signing block size fields don't match (corrupt APK?)")
	}
	totalLen := 8 + int(size1)
	if blockStart+totalLen > len(data) {
		return nil, fmt.Errorf("signing block extends past end of file")
	}

	return data[blockStart : blockStart+totalLen], nil
}

// findEOCD searches for the ZIP End Of Central Directory signature,
// scanning backward from the end of the file since it may be followed by
// up to 65535 bytes of zip file comment.
func findEOCD(data []byte) (int, error) {
	const eocdSig = 0x06054b50
	const minEOCDSize = 22
	const maxCommentSize = 65535

	if len(data) < minEOCDSize {
		return 0, fmt.Errorf("file too small to be a zip archive")
	}
	searchStart := len(data) - minEOCDSize - maxCommentSize
	if searchStart < 0 {
		searchStart = 0
	}
	for i := len(data) - minEOCDSize; i >= searchStart; i-- {
		if binary.LittleEndian.Uint32(data[i:]) == eocdSig {
			return i, nil
		}
	}
	return 0, fmt.Errorf("not a valid zip file (no End Of Central Directory record found)")
}

// parseSigningBlock walks the ID-value pairs of an APK Signing Block (the
// slice returned by findSigningBlock, i.e. including its leading/trailing
// size fields and magic), extracting signer certificates from the v2
// and/or v3 entries.
func parseSigningBlock(block []byte) (certs []CertInfo, hasV2, hasV3 bool, err error) {
	pos := 8                   // past the leading "size of block" field
	end := len(block) - 8 - 16 // exclude the trailing size field + magic
	if end < pos {
		return nil, false, false, fmt.Errorf("signing block too small to contain any ID-value pairs")
	}

	for pos < end {
		if pos+12 > end {
			return certs, hasV2, hasV3, fmt.Errorf("truncated ID-value pair at offset %d", pos)
		}
		pairLen := binary.LittleEndian.Uint64(block[pos:])
		id := binary.LittleEndian.Uint32(block[pos+8:])
		valueStart := pos + 12
		valueLen := int(pairLen) - 4
		if valueLen < 0 || valueStart+valueLen > end {
			return certs, hasV2, hasV3, fmt.Errorf("id-value pair 0x%08x has an invalid length", id)
		}
		value := block[valueStart : valueStart+valueLen]

		switch id {
		case idSignatureV2, idSignatureV3:
			if id == idSignatureV2 {
				hasV2 = true
			} else {
				hasV3 = true
			}
			found, extractErr := extractCertificates(value)
			certs = append(certs, found...)
			if extractErr != nil && err == nil {
				err = extractErr
			}
		}

		pos = valueStart + valueLen
	}
	return certs, hasV2, hasV3, err
}

// extractCertificates walks one v2/v3 signature scheme block's value: a
// length-prefixed sequence of "signer" records, each containing
// length-prefixed signed-data with, among other things, a length-prefixed
// sequence of raw X.509 DER certificates. See "APK Signature Scheme v2
// Block Format" at developer.android.com/apk/apk-signing.
func extractCertificates(value []byte) ([]CertInfo, error) {
	signerSeq, _, err := readLenPrefixed32(value, 0)
	if err != nil {
		return nil, fmt.Errorf("signer sequence: %w", err)
	}

	var certs []CertInfo
	pos := 0
	for pos < len(signerSeq) {
		signer, next, err := readLenPrefixed32(signerSeq, pos)
		if err != nil {
			return certs, fmt.Errorf("signer record: %w", err)
		}
		pos = next

		signedData, _, err := readLenPrefixed32(signer, 0)
		if err != nil {
			return certs, fmt.Errorf("signed-data: %w", err)
		}

		_, afterDigests, err := readLenPrefixed32(signedData, 0) // digests sequence, not needed for a summary
		if err != nil {
			return certs, fmt.Errorf("digests: %w", err)
		}
		certsBlob, _, err := readLenPrefixed32(signedData, afterDigests)
		if err != nil {
			return certs, fmt.Errorf("certificates: %w", err)
		}

		cpos := 0
		for cpos < len(certsBlob) {
			certDER, cnext, err := readLenPrefixed32(certsBlob, cpos)
			if err != nil {
				return certs, fmt.Errorf("certificate: %w", err)
			}
			cpos = cnext

			cert, err := x509.ParseCertificate(certDER)
			if err != nil {
				return certs, fmt.Errorf("parsing X.509 certificate: %w", err)
			}
			sum := sha256.Sum256(certDER)
			certs = append(certs, CertInfo{
				Subject:      cert.Subject.String(),
				Issuer:       cert.Issuer.String(),
				SerialNumber: cert.SerialNumber.String(),
				NotBefore:    cert.NotBefore.UTC().Format("2006-01-02"),
				NotAfter:     cert.NotAfter.UTC().Format("2006-01-02"),
				SHA256:       hex.EncodeToString(sum[:]),
			})
		}
	}
	return certs, nil
}

// readLenPrefixed32 reads a uint32 byte-length followed by that many bytes
// - the recurring encoding used throughout the v2/v3 signing block.
func readLenPrefixed32(data []byte, pos int) ([]byte, int, error) {
	if pos+4 > len(data) {
		return nil, pos, fmt.Errorf("truncated length prefix at offset %d", pos)
	}
	n := int(binary.LittleEndian.Uint32(data[pos:]))
	start := pos + 4
	if n < 0 || start+n > len(data) {
		return nil, pos, fmt.Errorf("length-prefixed blob at offset %d exceeds available data", pos)
	}
	return data[start : start+n], start + n, nil
}
