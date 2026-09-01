package youtube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectTrack(t *testing.T) {
	humanEN := Track{Lang: "en", Generated: false, BaseURL: "https://example.com/tt?lang=en"}
	asrEN := Track{Lang: "en", Generated: true, BaseURL: "https://example.com/tt?lang=en&kind=asr"}
	humanES := Track{Lang: "es", Generated: false, BaseURL: "https://example.com/tt?lang=es"}
	asrNL := Track{Lang: "nl", Generated: true, BaseURL: "https://example.com/tt?lang=nl"}

	tests := []struct {
		name    string
		tracks  []Track
		lang    string
		want    Track
		wantErr error
	}{
		{
			name:   "human en beats asr en",
			tracks: []Track{humanEN, asrEN},
			lang:   "en",
			want:   humanEN,
		},
		{
			name:   "human en beats first-track fallback",
			tracks: []Track{humanES, humanEN, asrEN},
			lang:   "en",
			want:   humanEN,
		},
		{
			name:   "asr en when no human en",
			tracks: []Track{humanES, asrEN},
			lang:   "en",
			want:   asrEN,
		},
		{
			name:   "fallback to first track when lang missing",
			tracks: []Track{humanES, asrNL},
			lang:   "fr",
			want:   humanES,
		},
		{
			name:   "single generated track selected by lang",
			tracks: []Track{asrNL},
			lang:   "nl",
			want:   asrNL,
		},
		{
			name:    "empty list is ErrNoCaptions",
			tracks:  nil,
			lang:    "en",
			wantErr: ErrNoCaptions,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectTrack(tt.tracks, tt.lang)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
