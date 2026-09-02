package message

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// TestBuildMIMERefusesControlCharacters pins the BuildMIME-entry fail-closed
// behavior: every compose path (send and draft) rejects CRLF-bearing values
// before any bytes are built.
func TestBuildMIMERefusesControlCharacters(t *testing.T) {
	tests := []struct {
		name string
		to   []string
		cc   []string
		bcc  []string
		sub  string
		want string
	}{
		{name: "to item", to: []string{"a@x.com", "b\r\nBcc: victim@evil.example"}, sub: "s", want: `recipient "b\r\nBcc: victim@evil.example" contains control characters`},
		{name: "cc item", cc: []string{"b@x.com\nX-Evil: 1"}, sub: "s", want: `recipient "b@x.com\nX-Evil: 1" contains control characters`},
		{name: "bcc item", bcc: []string{"b@x.com\rBcc: victim@evil.example"}, sub: "s", want: "contains control characters"},
		{name: "subject", to: []string{"a@x.com"}, sub: "hi\r\nBcc: victim@evil.example", want: `subject "hi\r\nBcc: victim@evil.example" contains control characters`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := BuildMIME(afero.NewMemMapFs(), tt.to, tt.cc, tt.bcc, tt.sub, "body", nil)
			require.Nil(t, raw, "no bytes may be built for rejected input")
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestBuildMIMEAttachmentNameValidation(t *testing.T) {
	fs := afero.NewMemMapFs()

	t.Run("double quote in name is rejected", func(t *testing.T) {
		require.NoError(t, afero.WriteFile(fs, `evil"name.txt`, []byte("data"), 0o644))
		raw, err := BuildMIME(fs, []string{"a@x.com"}, nil, nil, "s", "body", []string{`evil"name.txt`})
		require.Nil(t, raw)
		require.ErrorContains(t, err, `file name "evil\"name.txt" contains '"'`)
	})

	t.Run("LF in name is rejected", func(t *testing.T) {
		require.NoError(t, afero.WriteFile(fs, "evil\nname.txt", []byte("data"), 0o644))
		raw, err := BuildMIME(fs, []string{"a@x.com"}, nil, nil, "s", "body", []string{"evil\nname.txt"})
		require.Nil(t, raw)
		require.ErrorContains(t, err, `contains '\n'`)
	})

	t.Run("CR in name is rejected", func(t *testing.T) {
		require.NoError(t, afero.WriteFile(fs, "evil\rname.txt", []byte("data"), 0o644))
		raw, err := BuildMIME(fs, []string{"a@x.com"}, nil, nil, "s", "body", []string{"evil\rname.txt"})
		require.Nil(t, raw)
		require.ErrorContains(t, err, `contains '\r'`)
	})

	t.Run("clean names still build well-formed parts", func(t *testing.T) {
		require.NoError(t, afero.WriteFile(fs, "résumé.pdf", []byte("data"), 0o644))
		raw, err := BuildMIME(fs, []string{"a@x.com"}, nil, nil, "s", "body", []string{"résumé.pdf"})
		require.NoError(t, err)
		require.Contains(t, string(raw), `filename="résumé.pdf"`)
	})
}
