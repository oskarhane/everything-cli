package podcast

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeMatch(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "casefold and punctuation", in: "Hello, World!", want: "hello world"},
		{name: "keeps accents", in: "Caf\u00e9 au Lait", want: "café au lait"},
		{name: "curly quotes to ascii", in: "\u2018single\u2019 \u201cdouble\u201d", want: `'single' "double"`},
		{name: "dashes to hyphen", in: "A \u2013 B \u2014 C \u2015 D", want: "a - b - c - d"},
		{name: "ellipsis", in: "Wait\u2026", want: "wait..."},
		{name: "no-break space", in: "No\u00a0Break", want: "no break"},
		{name: "collapse whitespace", in: "  many   spaces\tand\nlines  ", want: "many spaces and lines"},
		{name: "drops emoji", in: "Hi \U0001f600 there", want: "hi there"},
		{name: "drops misc symbols", in: "Love \u2764 selector", want: "love selector"},
		{name: "drops variation selector", in: "X\ufe0f marks the spot", want: "x marks the spot"},
		{name: "drops zwj", in: "Family \U0001f468\u200d\U0001f469", want: "family"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeMatch(tt.in))
		})
	}
}

func TestNormalizeMatchTierTable(t *testing.T) {
	tests := []struct {
		name     string
		want     string
		titles   []string
		wantIdx  int
		wantErr  bool
		wantSent bool // whether to assert ErrEpisodeNotFound specifically
	}{
		{
			name:    "tier 1 exact raw equality",
			want:    "Ep 12: Growing Coffee",
			titles:  []string{"News Roundup", "Ep 12: Growing Coffee"},
			wantIdx: 1,
		},
		{
			name:    "tier 2 normalized typographic fold",
			want:    "Ep 12 \u2013 Growing Coffee, in the Andes!",
			titles:  []string{"Ep 12 - Growing Coffee, in the Andes", "Other episode"},
			wantIdx: 0,
		},
		{
			name:    "tier 2 normalized casefold",
			want:    "THE DEEP DIVE",
			titles:  []string{"the deep dive"},
			wantIdx: 0,
		},
		{
			name:    "tier 3 single containment",
			want:    "Growing Coffee in the Andes Mountains part two",
			titles:  []string{"Growing Coffee in the Andes Mountains"},
			wantIdx: 0,
		},
		{
			name:    "tier 3 single containment via single decoy-like candidate",
			want:    "Growing Coffee in the Andes Mountains",
			titles:  []string{"Essentials: Growing Coffee in the Andes Mountains"},
			wantIdx: 0,
		},
		{
			name:     "tier 3 shorter side under 10 runes rejected",
			want:     "Growing Coffee in the Andes Mountains part two",
			titles:   []string{"coffee"},
			wantErr:  true,
			wantSent: true,
		},
		{
			name:     "tier 3 ambiguity with essentials decoy must not false-positive",
			want:     "Growing Coffee in the Andes Mountains",
			titles:   []string{"Growing Coffee in the Andes Mountains (Repost)", "Essentials: Growing Coffee in the Andes Mountains"},
			wantErr:  true,
			wantSent: true,
		},
		{
			name:     "no match anywhere",
			want:     "Something entirely different about tea drinking habits",
			titles:   []string{"Coffee", "Tea"},
			wantErr:  true,
			wantSent: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matchTitle(tt.want, tt.titles)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantSent {
					require.ErrorIs(t, err, ErrEpisodeNotFound)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantIdx, got)
		})
	}
}
