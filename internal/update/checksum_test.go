package update

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sha256Hex(t *testing.T, data []byte) string {
	t.Helper()
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestParseChecksums(t *testing.T) {
	aaaa := strings.Repeat("a", 64)
	bbbb := strings.Repeat("b", 64)
	cccc := strings.Repeat("c", 64)
	tests := []struct {
		name string
		data string
		want map[string]string
	}{
		{
			name: "two-space text format",
			data: aaaa + "  google-cli_darwin_arm64.tar.gz\n" + bbbb + "  google-cli_linux_amd64.tar.gz\n",
			want: map[string]string{
				"google-cli_darwin_arm64.tar.gz": aaaa,
				"google-cli_linux_amd64.tar.gz":  bbbb,
			},
		},
		{
			name: "binary-marker format",
			data: aaaa + " *google-cli_darwin_arm64.tar.gz\n" + bbbb + "*google-cli_linux_amd64.tar.gz\n",
			want: map[string]string{
				"google-cli_darwin_arm64.tar.gz": aaaa,
				"google-cli_linux_amd64.tar.gz":  bbbb,
			},
		},
		{
			name: "mixed formats",
			data: aaaa + "  one.tar.gz\n" + bbbb + " *two.tar.gz\n" + cccc + " three.tar.gz\n",
			want: map[string]string{
				"one.tar.gz":   aaaa,
				"two.tar.gz":   bbbb,
				"three.tar.gz": cccc,
			},
		},
		{
			name: "skips garbage lines and empty lines",
			data: "not a checksum line\n\n" + aaaa + "  ok.tar.gz\n",
			want: map[string]string{"ok.tar.gz": aaaa},
		},
		{
			name: "empty input",
			data: "",
			want: map[string]string{},
		},
		{
			name: "mixed-case hex lower-cased",
			data: strings.ToUpper(aaaa) + "  x.tar.gz\n",
			want: map[string]string{"x.tar.gz": aaaa},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ParseChecksums([]byte(tt.data)))
		})
	}
}

func TestVerify(t *testing.T) {
	data := []byte("payload bytes")
	want := sha256Hex(t, data)

	t.Run("pass", func(t *testing.T) {
		require.NoError(t, Verify(data, want))
	})
	t.Run("pass case-insensitive", func(t *testing.T) {
		require.NoError(t, Verify(data, strings.ToUpper(want)))
	})
	t.Run("fail", func(t *testing.T) {
		other := sha256Hex(t, []byte("different bytes"))
		err := Verify(data, other)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrChecksumMismatch), "want ErrChecksumMismatch, got %v", err)
	})
	t.Run("fail malformed", func(t *testing.T) {
		err := Verify(data, "nothex")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrChecksumMismatch), "want ErrChecksumMismatch, got %v", err)
	})
}
