package podcast

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// normalizeMatch normalizes an episode title for tiered matching. Both the
// wanted title and every feed candidate are passed through this identically:
//  1. strings.ToLower (Unicode-aware);
//  2. explicit folding of typographic punctuation to ASCII via foldPunct;
//  3. dropping emoji (U+1F000+, U+2600-U+27BF, variation selectors, ZWJ);
//  4. dropping every remaining rune that is not a letter, digit, space, or
//     one of the ASCII punctuation characters foldPunct can produce;
//  5. collapsing whitespace via strings.Fields + single-space join.
//
// Step 4 keeps the ASCII punctuation emitted by step 2 so a typographic dash
// and an ASCII dash normalize identically (they both yield '-').
func normalizeMatch(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if ascii, ok := foldPunct(r); ok {
			b.WriteString(ascii)
			continue
		}
		switch {
		case r >= 0x1F000: // large supplementary emoji (U+1F000+)
			continue
		case r >= 0x2600 && r <= 0x27BF: // misc symbols / dingbats emoji range
			continue
		case r >= 0xFE00 && r <= 0xFE0F: // variation selectors
			continue
		case r == 0x200D: // zero-width joiner
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) ||
			r == '.' || r == '-' || r == '\'' || r == '"' {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// foldPunct maps typographic punctuation to its ASCII equivalent. The ASCII
// forms are kept during normalization (see normalizeMatch), so both a
// typographic and an ASCII dash fold to the same '-'.
func foldPunct(r rune) (string, bool) {
	switch r {
	case '\u2018', '\u2019', '\u201A', '\u201B': // curly/smart single quotes
		return "'", true
	case '\u201C', '\u201D', '\u201E': // curly/smart double quotes
		return "\"", true
	case '\u2013', '\u2014', '\u2015', '\u2212': // en/em/horizontal/minus dashes
		return "-", true
	case '\u2026': // ellipsis
		return "...", true
	case '\u00A0': // no-break space
		return " ", true
	}
	return "", false
}

// matchTitle finds the index of the feed item whose title matches want under
// the strict three-tier recipe, first hit wins:
//   - TIER 1: exact raw UTF-8 equality;
//   - TIER 2: exact normalized equality;
//   - TIER 3: containment (either normalized string contains the other),
//     requiring the shorter normalized side to be >= 10 runes, and ONLY
//     accepting when exactly one candidate matches. Any ambiguity, or no
//     match at all, yields ErrEpisodeNotFound.
func matchTitle(want string, titles []string) (int, error) {
	// TIER 1: exact raw UTF-8 equality.
	for i, t := range titles {
		if t == want {
			return i, nil
		}
	}

	// TIER 2: exact normalized equality (normalize once per candidate, reusing
	// the results for the tier-3 pass).
	wn := normalizeMatch(want)
	norm := make([]string, len(titles))
	for i, t := range titles {
		norm[i] = normalizeMatch(t)
		if norm[i] == wn {
			return i, nil
		}
	}

	// TIER 3: containment with the uniqueness requirement.
	hit := -1
	for i, tn := range norm {
		a, b := wn, tn
		if len(a) > len(b) {
			a, b = b, a
		}
		if utf8.RuneCountInString(a) >= 10 && strings.Contains(b, a) {
			if hit >= 0 {
				return 0, ErrEpisodeNotFound // ambiguous containment -> no match
			}
			hit = i
		}
	}
	if hit < 0 {
		return 0, ErrEpisodeNotFound
	}
	return hit, nil
}
