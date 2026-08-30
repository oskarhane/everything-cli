// Package skill embeds the google-cli agent-skill bundle and installs it
// into supported AI agents' skills directories.
//
// The committed bundle/ tree is the source of truth shipped in the binary.
package skill

import (
	"embed"
	"io/fs"
)

// rawBundle is the raw embed.FS rooted ABOVE the bundle/ directory. Do not
// expose this directly to the installer — `fs.WalkDir(rawBundle, ".")` would
// yield `bundle/SKILL.md`, not `SKILL.md`, and the installer assumes the
// flat layout (SKILL.md + references/ at the root).
//
//go:embed bundle
var rawBundle embed.FS

// Bundle is the agent-skill bundle rooted at the bundle/ contents, so
// `fs.WalkDir(Bundle, ".")` yields `SKILL.md` and `references/<sub>.md` at
// the root — the flat layout the installer expects.
var Bundle fs.FS = mustSub(rawBundle, "bundle")

// SkillName is the on-disk directory name for the installed bundle inside
// each agent's skills directory.
const SkillName = "google-cli"

// mustSub wraps fs.Sub and panics on error. The only failure modes are
// programmer errors (invalid path), so a panic at init time is acceptable.
func mustSub(parent fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(parent, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
