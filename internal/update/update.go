// Package update — Run/Check orchestration for the self-update flow.
//
// Check compares the installed version against the latest release. Run drives
// the full update: verify the release artifact against its checksum manifest,
// replace the running binary atomically, then re-install the skill bundle by
// exec'ing the freshly replaced binary as a subprocess (the running process
// still holds the old embedded bundle, so an in-process install would
// resurrect stale skill content).
package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/spf13/afero"

	"github.com/oskarhane/google-cli/internal/skill"
)

// ErrUpToDate is returned by Run when the installed version already matches
// the latest release. Run still returns a populated Result alongside the
// error; the command layer maps errors.Is(err, ErrUpToDate) to exit 0 with
// updated:false.
var ErrUpToDate = errors.New("already up to date")

const (
	// checksumsAsset is the release asset holding the sha256sum-style
	// manifest for the tarball.
	checksumsAsset = "checksums.txt"
	// binaryName is the tarball member holding the binary.
	binaryName = "google-cli"
	// DefaultRepoSlug is the repo checked when Options.RepoSlug is empty.
	DefaultRepoSlug = defaultRepo
)

// execSkillInstallFn is the Options.ExecSkillInstall signature: run the
// resolved self path as a subprocess with the given arguments.
type execSkillInstallFn = func(ctx context.Context, args []string) ([]byte, error)

// replaceBinary is a seam over ReplaceBinary so orchestrator tests can
// observe the replacement call without touching a real binary.
var replaceBinary = ReplaceBinary

// Check reports the latest release and whether an update is available.
// Update-available rule: a current version that fails ParseVersion (dev or
// git-SHA local build) is ALWAYS update-available; otherwise available
// means Compare(latest, current) > 0 (equal or older is not). ErrNoReleases
// and ErrRateLimited from the client propagate unchanged.
func Check(ctx context.Context, client Client, current string) (*Release, bool, error) {
	rel, err := client.LatestRelease(ctx)
	if err != nil {
		return nil, false, err
	}
	cur, ok := ParseVersion(current)
	if !ok {
		// Dev/SHA local build: the release is by definition newer.
		return rel, true, nil
	}
	latest, ok := ParseVersion(rel.Tag)
	if !ok {
		return rel, false, fmt.Errorf("release tag %q is not a semantic version", rel.Tag)
	}
	return rel, Compare(latest, cur) > 0, nil
}

// Options configures Run.
type Options struct {
	// RepoSlug is owner/repo to check. Empty means DefaultRepoSlug. Run
	// never reads it: the caller resolves it into the Client before
	// calling Run. Carried here so the command layer passes one struct.
	RepoSlug string
	// AgentFilter restricts the skill reinstall to one agent ("" installs
	// into every detected agent). Forwarded to the subprocess as
	// ["--agent", AgentFilter].
	AgentFilter string
	// Yes is the --yes flag. Run does NOT act on it: the install decision
	// belongs to the caller (see SkipSkillInstall).
	Yes bool
	// CheckOnly stops Run after the version comparison: no downloads, no
	// replacement, no skill install.
	CheckOnly bool
	// SkipSkillInstall suppresses the post-update skill reinstall. Zero
	// value (false) means Run ALWAYS reinstalls the skill bundle after a
	// successful binary replacement — the install decision itself belongs
	// to the caller, which pre-decides it via prompt/flag (node 08's
	// contract: --yes auto-installs, prompt on TTY, hint otherwise; the
	// caller sets SkipSkillInstall only when the user declined or is not
	// present to be prompted). When skipped, Run fills skill_hint.
	SkipSkillInstall bool
	// FS is the filesystem used to detect agents and read the installed
	// SKILL.md. Zero value uses afero.NewOsFs; tests inject afero.NewMemMapFs.
	FS afero.Fs
	// ExecSkillInstall runs the replaced binary as a subprocess. Zero value
	// uses the real os/exec implementation (execSkillInstall).
	ExecSkillInstall execSkillInstallFn
}

// Result is the structured output of Run. Field names are snake_case in
// output (JSON/TOON keys, table headers) via Fields()/Row() + output.Print.
type Result struct {
	CurrentVersion  string   `json:"current_version"`
	LatestVersion   string   `json:"latest_version"`
	UpdateAvailable bool     `json:"update_available"`
	LocalBuild      bool     `json:"local_build"`
	Updated         bool     `json:"updated"`
	BinaryPath      string   `json:"binary_path"`
	SkillInstalled  []string `json:"skill_installed"`
	SkillVersion    string   `json:"skill_version"`
	SkillHint       string   `json:"skill_hint"`
}

// Fields is the table-field order for table-format output.
func (r Result) Fields() []string {
	return []string{
		"current_version", "latest_version", "update_available", "local_build",
		"updated", "binary_path", "skill_installed", "skill_version", "skill_hint",
	}
}

// Row renders the result as the single table row for output.Print.
func (r Result) Row() map[string]any {
	return map[string]any{
		"current_version":  r.CurrentVersion,
		"latest_version":   r.LatestVersion,
		"update_available": r.UpdateAvailable,
		"local_build":      r.LocalBuild,
		"updated":          r.Updated,
		"binary_path":      r.BinaryPath,
		"skill_installed":  r.SkillInstalled,
		"skill_version":    r.SkillVersion,
		"skill_hint":       r.SkillHint,
	}
}

// Run orchestrates the self-update flow.
//
// The skill-install decision is made by the CALLER: Run always reinstalls
// the skill bundle after a successful binary replacement unless
// opts.SkipSkillInstall is set (the caller decides via --yes/prompt/hint —
// see the SkipSkillInstall doc). Run cannot prompt: it may run non-interactively.
//
// Error contract (the command layer maps these):
//   - ErrUpToDate (wrapped): current is already the latest; the returned
//     Result carries the version comparison and updated=false, so the
//     caller can errors.Is(ErrUpToDate) -> exit 0 while still rendering.
//   - ErrNoReleases / ErrRateLimited from LatestRelease propagate unchanged.
//   - Download-phase failures are wrapped with the asset name ("downloading
//     <asset>"): the shared client maps 404/403 to sentinels regardless of
//     phase, so the wrapper is what disambiguates a failed asset download
//     from "no releases published yet".
//   - ErrChecksumMismatch aborts before the installed binary is touched.
//   - If the binary was replaced but the skill reinstall failed, the error
//     says so and the Result still reports updated=true and binary_path.
func Run(ctx context.Context, client Client, current string, opts Options) (Result, error) {
	rel, available, err := Check(ctx, client, current)
	if err != nil {
		return Result{}, err
	}
	localBuild := false
	if _, ok := ParseVersion(current); !ok {
		localBuild = true
	}

	res := Result{
		CurrentVersion:  current,
		LatestVersion:   rel.Tag,
		UpdateAvailable: available,
		LocalBuild:      localBuild,
	}
	binPath, err := resolvedSelfPath()
	if err != nil {
		return res, err
	}
	res.BinaryPath = binPath

	if opts.CheckOnly {
		return res, nil
	}
	if !available {
		return res, fmt.Errorf("%w (%s is current)", ErrUpToDate, current)
	}
	return performUpdate(ctx, client, rel, current, opts, res)
}

// performUpdate runs the download/verify/replace/reinstall pipeline for an
// available update. res carries the already-populated comparison fields.
func performUpdate(ctx context.Context, client Client, rel *Release, current string, opts Options, res Result) (Result, error) {
	tarAsset, err := rel.Asset(assetName(runtime.GOOS, runtime.GOARCH))
	if err != nil {
		return res, fmt.Errorf("release %s: %w", rel.Tag, err)
	}
	sumAsset, err := rel.Asset(checksumsAsset)
	if err != nil {
		return res, fmt.Errorf("release %s: %w", rel.Tag, err)
	}

	tarData, err := client.Download(ctx, tarAsset.URL)
	if err != nil {
		return res, fmt.Errorf("downloading %s: %w", tarAsset.Name, err)
	}
	sumData, err := client.Download(ctx, sumAsset.URL)
	if err != nil {
		return res, fmt.Errorf("downloading %s: %w", sumAsset.Name, err)
	}

	want, ok := ParseChecksums(sumData)[tarAsset.Name]
	if !ok {
		return res, fmt.Errorf("checksums.txt has no entry for %s", tarAsset.Name)
	}
	// Verify BEFORE touching the installed binary: a tampered or truncated
	// artifact must never reach ReplaceBinary.
	if err := Verify(tarData, want); err != nil {
		return res, fmt.Errorf("verifying %s: %w", tarAsset.Name, err)
	}

	tmp, err := extractBinary(tarData)
	if err != nil {
		return res, err
	}
	// The staged binary is removed on every exit path, success included.
	defer func() { _ = os.Remove(tmp) }()

	if err := replaceBinary(tmp); err != nil {
		return res, fmt.Errorf("replacing %s: %w", filepath.Base(res.BinaryPath), err)
	}
	res.Updated = true

	if opts.SkipSkillInstall {
		res.SkillHint = "run 'google-cli skill install' to refresh the installed skill bundle"
		return res, nil
	}
	return reinstallSkill(ctx, opts, res)
}

// resolvedSelfPath is the symlink-resolved path of the running binary — the
// file ReplaceBinary targets and the executable the skill reinstall execs.
func resolvedSelfPath() (string, error) {
	self, err := SelfPath()
	if err != nil {
		return "", fmt.Errorf("resolving current executable: %w", err)
	}
	target, err := filepath.EvalSymlinks(self)
	if err != nil {
		return "", fmt.Errorf("resolving binary path %s: %w", self, err)
	}
	return target, nil
}

// extractBinary pulls the google-cli member out of the release tar.gz into a
// temp file with mode 0755. The caller removes the temp file.
func extractBinary(tarGz []byte) (string, error) {
	gz, err := gzip.NewReader(bytes.NewReader(tarGz))
	if err != nil {
		return "", fmt.Errorf("reading release tarball: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading release tarball: %w", err)
		}
		// Exact member match only: filepath.Clean also collapses "./google-cli",
		// and any traversal-style name cannot equal "google-cli".
		if hdr.Typeflag != tar.TypeReg || filepath.Clean(hdr.Name) != binaryName {
			continue
		}
		return stageBinary(tr)
	}
	return "", fmt.Errorf("release tarball has no %s member", binaryName)
}

// stageBinary writes a tar entry to a fresh temp file with executable
// permissions. The temp file is removed on any failure.
func stageBinary(tr *tar.Reader) (string, error) {
	tmp, err := os.CreateTemp("", "google-cli-update-*")
	if err != nil {
		return "", fmt.Errorf("staging new binary: %w", err)
	}
	path := tmp.Name()
	if _, err := io.Copy(tmp, tr); err != nil {
		_ = tmp.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("extracting %s: %w", binaryName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("extracting %s: %w", binaryName, err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("setting executable permissions: %w", err)
	}
	return path, nil
}

// reinstallSkill installs the skill bundle by exec'ing the freshly replaced
// binary (NEVER in-process — this process still holds the old embedded
// bundle), then reads the installed SKILL.md back to report the version.
func reinstallSkill(ctx context.Context, opts Options, res Result) (Result, error) {
	execFn := opts.ExecSkillInstall
	if execFn == nil {
		execFn = execSkillInstall
	}
	args := []string{"skill", "install"}
	if opts.AgentFilter != "" {
		args = append(args, "--agent", opts.AgentFilter)
	}
	if _, err := execFn(ctx, args); err != nil {
		return res, fmt.Errorf("binary replaced, but skill reinstall failed: %w", err)
	}

	fs := opts.FS
	if fs == nil {
		fs = afero.NewOsFs()
	}
	paths, version, err := installedSkill(fs, opts.AgentFilter)
	if err != nil {
		return res, fmt.Errorf("binary replaced, but verifying skill install: %w", err)
	}
	res.SkillInstalled = paths
	res.SkillVersion = version
	return res, nil
}

// installedSkill reports the installed google-cli bundle directories and the
// version stamped in the first installed SKILL.md. Read-only mirror of
// skill.Install's target resolution against the post-install state.
func installedSkill(fs afero.Fs, agentFilter string) ([]string, string, error) {
	agents := skill.DetectAgents(fs)
	if agentFilter != "" {
		a := skill.FindAgent(agentFilter)
		if a == nil {
			return nil, "", fmt.Errorf("%w: %q", skill.ErrUnknownAgent, agentFilter)
		}
		agents = []skill.Agent{*a}
	}

	var paths []string
	version := ""
	for _, a := range agents {
		root, ok := a.SkillsPath()
		if !ok {
			continue
		}
		dir := filepath.Join(root, skill.SkillName)
		data, err := afero.ReadFile(fs, filepath.Join(dir, "SKILL.md"))
		if err != nil {
			continue // agent detected but bundle absent
		}
		paths = append(paths, dir)
		if version == "" {
			version = parseSkillVersion(data)
		}
	}
	return paths, version, nil
}

// skillVersionRe matches the frontmatter `version:` line of an installed
// SKILL.md (same tolerances as the skill package's matcher).
var skillVersionRe = regexp.MustCompile(`(?m)^[ \t]*version:[ \t]*([^\r\n]*?)[ \t]*\r?$`)

func parseSkillVersion(data []byte) string {
	m := skillVersionRe.FindSubmatch(data)
	if m == nil {
		return ""
	}
	return string(m[1])
}

// execSkillInstall is the default ExecSkillInstall: run the resolved self
// path (the just-replaced binary) as a subprocess with the given arguments.
func execSkillInstall(ctx context.Context, args []string) ([]byte, error) {
	bin, err := resolvedSelfPath()
	if err != nil {
		return nil, err
	}
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("running %s %s: %w", bin, strings.Join(args, " "), err)
	}
	return out, nil
}
