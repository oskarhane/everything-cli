package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/skill"
)

// fakeClient is an in-memory Client: release metadata plus a URL to body
// map, with every Download call recorded.
type fakeClient struct {
	rel        *Release
	bodies     map[string][]byte
	latestErr  error
	downloaded []string
}

func (f *fakeClient) LatestRelease(context.Context) (*Release, error) {
	if f.latestErr != nil {
		return nil, f.latestErr
	}
	return f.rel, nil
}

func (f *fakeClient) Download(_ context.Context, url string) ([]byte, error) {
	f.downloaded = append(f.downloaded, url)
	body, ok := f.bodies[url]
	if !ok {
		return nil, fmt.Errorf("unexpected download %q", url)
	}
	return body, nil
}

// env is the hermetic fixture for Run tests: a fake client, recording seams
// over ReplaceBinary and ExecSkillInstall, and an in-memory skill FS.
type env struct {
	client   *fakeClient
	fs       afero.Fs
	replaced []string   // ReplaceBinary seam invocations (staged paths)
	payload  []byte     // content of the staged file at replace time
	executed [][]string // ExecSkillInstall arg slices
	execErr  error      // error returned by the exec seam
}

const (
	testTag  = "v1.1.0"
	testHome = "/home/tester"
)

// newEnv wires the update seams hermetically: SelfPath points at a fake
// installed binary (through an install symlink, like a real install),
// ReplaceBinary is a recording no-op, and the skill FS is an empty in-memory
// FS whose agent dirs resolve under testHome via $HOME. Returns the env and
// the resolved fake binary target path.
func newEnv(t *testing.T) (*env, string) {
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, "everything-cli")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o755))
	link := filepath.Join(dir, "install-link")
	require.NoError(t, os.Symlink(target, link))

	origSelf := SelfPath
	SelfPath = func() (string, error) { return link, nil }
	t.Cleanup(func() { SelfPath = origSelf })

	e := &env{fs: afero.NewMemMapFs()}
	origReplace := replaceBinary
	replaceBinary = func(newPath string) error {
		data, err := os.ReadFile(newPath)
		if err != nil {
			return err
		}
		e.replaced = append(e.replaced, newPath)
		e.payload = data
		return nil
	}
	t.Cleanup(func() { replaceBinary = origReplace })

	t.Setenv("HOME", testHome)
	return e, target
}

// opts returns Options bound to this env's seams and FS.
func (e *env) opts() Options {
	return Options{FS: e.fs, ExecSkillInstall: e.exec}
}

// exec is the recording ExecSkillInstall seam.
func (e *env) exec(_ context.Context, args []string) ([]byte, error) {
	e.executed = append(e.executed, args)
	if e.execErr != nil {
		return nil, e.execErr
	}
	return []byte("installed"), nil
}

// seedSkill seeds the skill side: claude-code detected, SKILL.md installed
// at the given version.
func (e *env) seedSkill(t *testing.T, version string) {
	t.Helper()
	skillsDir := filepath.Join(testHome, ".claude", "skills", skill.SkillName)
	require.NoError(t, e.fs.MkdirAll(skillsDir, 0o755))
	body := "---\nname: google-cli\nversion: " + version + "\n---\nUse google-cli.\n"
	require.NoError(t, afero.WriteFile(e.fs, filepath.Join(skillsDir, "SKILL.md"), []byte(body), 0o644))
}

// testRelease builds a v1.1.0 release: a tarball with an everything-cli member
// (payload "NEW-PAYLOAD") plus a reference file, and a matching
// checksums.txt. wrongInner swaps the binary member for a misnamed one;
// badSum records a wrong digest in the manifest.
func testRelease(t *testing.T, wrongInner, badSum bool) (*Release, map[string][]byte) {
	t.Helper()

	tarName := AssetName(runtime.GOOS, runtime.GOARCH)
	files := map[string][]byte{
		binaryName:               []byte("NEW-PAYLOAD"),
		"references/overview.md": []byte("docs"),
	}
	if wrongInner {
		delete(files, binaryName)
		files["not-everything-cli"] = []byte("NEW-PAYLOAD")
	}
	tb := buildTarball(t, files)
	sum := sha256.Sum256(tb)
	digest := hex.EncodeToString(sum[:])
	if badSum {
		digest = "0000000000000000000000000000000000000000000000000000000000000000"
	}
	checksums := fmt.Sprintf("%s  %s\n", digest, tarName)

	tarURL := "https://releases.test/" + tarName
	sumURL := "https://releases.test/" + checksumsAsset
	rel := &Release{
		Tag: testTag,
		Assets: []Asset{
			{Name: tarName, URL: tarURL},
			{Name: checksumsAsset, URL: sumURL},
		},
	}
	bodies := map[string][]byte{tarURL: tb, sumURL: []byte(checksums)}
	return rel, bodies
}

// buildTarball gzips a tar archive of the given name-to-content map in a
// stable order.
func buildTarball(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, name := range []string{binaryName, "references/overview.md"} {
		data, ok := files[name]
		if !ok {
			continue
		}
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data))}))
		_, err := tw.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func TestCheck(t *testing.T) {
	ctx := context.Background()
	rel := &Release{Tag: "v1.1.0"}

	tests := []struct {
		name      string
		current   string
		rel       *Release
		latestErr error
		wantAvail bool
		wantTag   string
		wantErr   error
	}{
		{name: "newer release", current: "v1.0.0", rel: rel, wantAvail: true, wantTag: testTag},
		{name: "equal", current: testTag, rel: rel, wantAvail: false, wantTag: testTag},
		{name: "installed newer", current: "v2.0.0", rel: rel, wantAvail: false, wantTag: testTag},
		{name: "numeric compare", current: "v1.1.0", rel: &Release{Tag: "v1.10.0"}, wantAvail: true, wantTag: "v1.10.0"},
		{name: "prerelease older than release", current: "v1.1.0", rel: &Release{Tag: "v1.1.0-rc.1"}, wantAvail: false, wantTag: "v1.1.0-rc.1"},
		{name: "dev build always available", current: "dev", rel: rel, wantAvail: true, wantTag: testTag},
		{name: "sha build always available", current: "83f5d73", rel: rel, wantAvail: true, wantTag: testTag},
		{name: "no releases", current: "v1.0.0", latestErr: ErrNoReleases, wantErr: ErrNoReleases},
		{name: "rate limited", current: "v1.0.0", latestErr: ErrRateLimited, wantErr: ErrRateLimited},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &fakeClient{rel: tt.rel, latestErr: tt.latestErr}
			got, avail, err := Check(ctx, c, tt.current)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)
				assert.False(t, avail)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantAvail, avail)
			assert.Equal(t, tt.wantTag, got.Tag)
		})
	}
}

func TestCheckNonSemanticReleaseTag(t *testing.T) {
	c := &fakeClient{rel: &Release{Tag: "not-a-version"}, bodies: map[string][]byte{}}
	got, avail, err := Check(context.Background(), c, "v1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not-a-version")
	assert.False(t, avail)
	assert.NotNil(t, got)
}

func TestRunUpToDate(t *testing.T) {
	e, _ := newEnv(t)
	rel, bodies := testRelease(t, false, false)
	e.client = &fakeClient{rel: rel, bodies: bodies}

	res, err := Run(context.Background(), e.client, testTag, e.opts())
	require.ErrorIs(t, err, ErrUpToDate)
	assert.False(t, res.Updated)
	assert.False(t, res.UpdateAvailable)
	assert.False(t, res.LocalBuild)
	assert.Equal(t, testTag, res.CurrentVersion)
	assert.Equal(t, testTag, res.LatestVersion)

	// Zero downloads: nothing was fetched beyond the release metadata.
	assert.Empty(t, e.client.downloaded)
	assert.Empty(t, e.replaced)
	assert.Empty(t, e.executed)
}

func TestRunCheckOnly(t *testing.T) {
	e, _ := newEnv(t)
	rel, bodies := testRelease(t, false, false)
	e.client = &fakeClient{rel: rel, bodies: bodies}
	e.seedSkill(t, "1.1.0")

	res, err := Run(context.Background(), e.client, "v1.0.0", Options{FS: e.fs, ExecSkillInstall: e.exec, CheckOnly: true})
	require.NoError(t, err)

	assert.True(t, res.UpdateAvailable)
	assert.False(t, res.Updated, "check-only must not replace")
	assert.Empty(t, e.client.downloaded, "check-only must not download")
	assert.Empty(t, e.replaced)
	assert.Empty(t, e.executed)
	assert.NotEmpty(t, res.BinaryPath)
}

func TestRunFullUpdate(t *testing.T) {
	for _, tt := range []struct {
		name      string
		current   string
		agent     string
		wantArgs  []string
		wantLocal bool
	}{
		{name: "release version", current: "v1.0.0", wantArgs: []string{"skill", "install"}},
		{name: "agent filter", current: "v1.0.0", agent: "claude-code",
			wantArgs: []string{"skill", "install", "--agent", "claude-code"}},
		{name: "dev build proceeds", current: "dev", wantArgs: []string{"skill", "install"}, wantLocal: true},
		{name: "sha build proceeds", current: "83f5d73", wantArgs: []string{"skill", "install"}, wantLocal: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e, target := newEnv(t)
			rel, bodies := testRelease(t, false, false)
			e.client = &fakeClient{rel: rel, bodies: bodies}
			e.seedSkill(t, "1.1.0")
			opts := e.opts()
			opts.AgentFilter = tt.agent

			res, err := Run(context.Background(), e.client, tt.current, opts)
			require.NoError(t, err)

			assert.True(t, res.Updated)
			assert.Equal(t, tt.wantLocal, res.LocalBuild)
			assert.Equal(t, tt.current, res.CurrentVersion)
			assert.Equal(t, testTag, res.LatestVersion)
			assert.True(t, res.UpdateAvailable)
			resolved, err := filepath.EvalSymlinks(target)
			require.NoError(t, err)
			assert.Equal(t, resolved, res.BinaryPath, "binary path is the resolved install target")
			assert.Equal(t, "NEW-PAYLOAD", string(e.payload), "staged binary content")
			require.Len(t, e.replaced, 1)
			_, statErr := os.Stat(e.replaced[0])
			assert.True(t, os.IsNotExist(statErr), "staged temp file must be removed after replace")
			assert.Equal(t, [][]string{tt.wantArgs}, e.executed)
			assert.Equal(
				t,
				[]string{filepath.Join(testHome, ".claude", "skills", skill.SkillName)},
				res.SkillInstalled,
			)
			assert.Equal(t, "1.1.0", res.SkillVersion)
			assert.Empty(t, res.SkillHint)
			assert.Len(t, e.client.downloaded, 2, "tarball + checksums fetched")
		})
	}
}

func TestRunSkipSkillInstall(t *testing.T) {
	e, _ := newEnv(t)
	rel, bodies := testRelease(t, false, false)
	e.client = &fakeClient{rel: rel, bodies: bodies}
	e.seedSkill(t, "1.1.0")

	opts := e.opts()
	opts.SkipSkillInstall = true
	res, err := Run(context.Background(), e.client, "v1.0.0", opts)
	require.NoError(t, err)

	assert.True(t, res.Updated)
	assert.Empty(t, e.executed, "skip means no subprocess install")
	assert.Empty(t, res.SkillInstalled)
	assert.Empty(t, res.SkillVersion)
	assert.Contains(t, res.SkillHint, "skill install")
}

func TestRunChecksumMismatchAbortsBeforeReplace(t *testing.T) {
	e, _ := newEnv(t)
	rel, bodies := testRelease(t, false, true)
	e.client = &fakeClient{rel: rel, bodies: bodies}

	res, err := Run(context.Background(), e.client, "v1.0.0", e.opts())
	require.ErrorIs(t, err, ErrChecksumMismatch)

	assert.Empty(t, e.replaced, "mismatch must abort before ReplaceBinary")
	assert.Empty(t, e.executed, "mismatch must abort before skill install")
	assert.False(t, res.Updated)
	assert.Len(t, e.client.downloaded, 2, "both artifacts fetched before verification failed")
}

func TestRunTarballWrongInnerName(t *testing.T) {
	e, _ := newEnv(t)
	rel, bodies := testRelease(t, true, false)
	e.client = &fakeClient{rel: rel, bodies: bodies}

	_, err := Run(context.Background(), e.client, "v1.0.0", e.opts())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no everything-cli member")
	assert.Empty(t, e.replaced)
	assert.Empty(t, e.executed)
}

func TestRunMissingTarballAsset(t *testing.T) {
	e, _ := newEnv(t)
	e.client = &fakeClient{rel: &Release{
		Tag:    testTag,
		Assets: []Asset{{Name: checksumsAsset, URL: "https://releases.test/" + checksumsAsset}},
	}, bodies: map[string][]byte{}}

	_, err := Run(context.Background(), e.client, "v1.0.0", e.opts())
	require.ErrorIs(t, err, ErrAssetNotFound)
	assert.Empty(t, e.replaced)
}

func TestRunMissingChecksumEntry(t *testing.T) {
	e, _ := newEnv(t)
	rel, bodies := testRelease(t, false, false)
	tarName := AssetName(runtime.GOOS, runtime.GOARCH)
	sumURL := "https://releases.test/" + checksumsAsset
	bodies[sumURL] = []byte(fmt.Sprintf("%s  other-file\n", "1111111111111111111111111111111111111111111111111111111111111111"))
	e.client = &fakeClient{rel: rel, bodies: bodies}

	_, err := Run(context.Background(), e.client, "v1.0.0", e.opts())
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrChecksumMismatch)
	assert.Contains(t, err.Error(), "no entry for "+tarName)
	assert.Empty(t, e.replaced)
}

func TestRunDownloadFailureNamesAsset(t *testing.T) {
	e, _ := newEnv(t)
	tarName := AssetName(runtime.GOOS, runtime.GOARCH)
	e.client = &fakeClient{rel: &Release{
		Tag: testTag,
		Assets: []Asset{
			{Name: tarName, URL: "https://releases.test/" + tarName},
			{Name: checksumsAsset, URL: "https://releases.test/" + checksumsAsset},
		},
	}, bodies: map[string][]byte{
		// checksums body present, tarball body missing: the fake errors out.
		"https://releases.test/" + checksumsAsset: []byte("x"),
	}}

	_, err := Run(context.Background(), e.client, "v1.0.0", e.opts())
	require.Error(t, err)
	// Reviewer-note contract: a download-phase failure names the asset, so
	// the message is not just the phase-blind sentinel.
	assert.Contains(t, err.Error(), "downloading "+tarName)
	assert.Empty(t, e.replaced)
}

func TestRunNoReleasesPropagates(t *testing.T) {
	e, _ := newEnv(t)
	e.client = &fakeClient{latestErr: ErrNoReleases}

	_, err := Run(context.Background(), e.client, "v1.0.0", e.opts())
	require.ErrorIs(t, err, ErrNoReleases)
}

func TestRunSkillInstallFailure(t *testing.T) {
	e, _ := newEnv(t)
	rel, bodies := testRelease(t, false, false)
	e.client = &fakeClient{rel: rel, bodies: bodies}
	e.execErr = errors.New("subprocess failed")

	res, err := Run(context.Background(), e.client, "v1.0.0", e.opts())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill")
	assert.True(t, res.Updated, "binary replacement stands even when skill install fails")
	assert.NotEmpty(t, res.BinaryPath)
}
