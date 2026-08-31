package update

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ErrChecksumMismatch is returned by Verify when the data digest does not
// match the expected checksum.
var ErrChecksumMismatch = errors.New("checksum mismatch")

const hexLen = 64

// ParseChecksums parses sha256sum-style checksum data into a filename to
// sha256-hex map. It tolerates both the two-space text format
// ("<hex>  <filename>") and the binary-marker format ("<hex> *<filename>");
// a single space between hash and filename is also accepted.
func ParseChecksums(data []byte) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) < hexLen+2 {
			continue
		}
		hexPart := strings.ToLower(line[:hexLen])
		if !isHex(line[:hexLen]) {
			continue
		}
		rest := line[hexLen:]
		if rest[0] != ' ' && rest[0] != '*' {
			continue
		}
		rest = strings.TrimLeft(rest, " ")
		rest = strings.TrimPrefix(rest, "*") // binary-mode marker
		name := strings.TrimSpace(rest)
		if name == "" {
			continue
		}
		out[name] = hexPart
	}
	return out
}

// Verify reports whether sha256(data) equals wantHex, case-insensitively.
func Verify(data []byte, wantHex string) error {
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, strings.TrimSpace(wantHex)) {
		return fmt.Errorf("%w: got %s", ErrChecksumMismatch, got)
	}
	return nil
}

func isHex(s string) bool {
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}
