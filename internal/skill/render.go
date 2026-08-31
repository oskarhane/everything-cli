package skill

import (
	"fmt"
	"io"
	"io/fs"
	"slices"
)

// Render writes the whole embedded bundle to w exactly as `skill print`
// emits it: the raw SKILL.md bytes first, then every references/*.md in
// sorted order, each preceded by a `===== references/<name>.md =====`
// separator line. It is the single owner of that rendering loop — the
// print command and the drift guard both consume it.
//
// Render performs straight writes; it deliberately bypasses any output
// formatting: the bundle is markdown and must never be marshalled.
func Render(w io.Writer) error {
	data, err := fs.ReadFile(Bundle, "SKILL.md")
	if err != nil {
		return fmt.Errorf("reading embedded SKILL.md: %w", err)
	}
	if _, werr := w.Write(data); werr != nil {
		return fmt.Errorf("writing SKILL.md: %w", werr)
	}

	refs, err := fs.Glob(Bundle, "references/*.md")
	if err != nil {
		return fmt.Errorf("listing embedded references: %w", err)
	}
	// fs.Glob on embed.FS returns sorted paths, but the contract is
	// explicit enough to not rely on that guarantee.
	slices.Sort(refs)

	for _, name := range refs {
		data, rerr := fs.ReadFile(Bundle, name)
		if rerr != nil {
			return fmt.Errorf("reading embedded %s: %w", name, rerr)
		}
		if _, werr := fmt.Fprintf(w, "===== %s =====\n", name); werr != nil {
			return fmt.Errorf("writing %s header: %w", name, werr)
		}
		if _, werr := w.Write(data); werr != nil {
			return fmt.Errorf("writing %s: %w", name, werr)
		}
	}
	return nil
}
