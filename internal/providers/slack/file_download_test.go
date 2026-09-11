package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testFileID is the file id every file download fixture answers for.
const testFileID = "F0B3HMXFEUV"

// testToken is the bearer token the simulated auth client stamps, so the
// bytes endpoint can prove the download carried the strategy's
// Authorization header.
const testToken = "xoxp-test-token"

// authTransport simulates the API-key strategy's authenticated client: it
// stamps Authorization: Bearer on every request before delegating to the
// httptest transport. The download must reuse this client, not a bare one.
type authTransport struct {
	base  http.RoundTripper
	token string
}

func (a *authTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.Header.Set("Authorization", "Bearer "+a.token)
	base := a.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(clone)
}

// fileRecorder records what the file endpoints saw: the Authorization header
// on the bytes request and the query params of files.info.
type fileRecorder struct {
	mu          sync.Mutex
	auth        string
	bytesHits   int
	infoQueries []url.Values
}

func (r *fileRecorder) recordBytes(auth string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.auth = auth
	r.bytesHits++
}

func (r *fileRecorder) recordInfo(q url.Values) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.infoQueries = append(r.infoQueries, q)
}

func (r *fileRecorder) authHeader() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.auth
}

// fileServerConfig tunes one hermetic file endpoint pair.
type fileServerConfig struct {
	content       []byte
	contentStatus int    // 0 => 200
	retryAfter    string // 429 only
	infoStatus    int    // 0 => 200
	infoBody      string // overrides the generated ok:true body when non-empty
	noURL         bool   // omit url_private
	redirectTo    string // bytes endpoint 302s here instead of serving
	keepURLPolicy bool   // don't stub validateFileURL (rejection tests)
}

// newFileServer serves the two endpoints a download touches: /files.info
// (under baseURL) and the file bytes at the absolute url_private. The
// generated files.info response points url_private back at this server, so
// the download's second, absolute request stays hermetic.
func newFileServer(t *testing.T, cfg fileServerConfig) (*httptest.Server, *fileRecorder) {
	t.Helper()
	rec := &fileRecorder{}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/files.info":
			rec.recordInfo(r.URL.Query())
			if cfg.infoStatus != 0 {
				w.WriteHeader(cfg.infoStatus)
			}
			w.Header().Set("Content-Type", "application/json")
			if cfg.infoBody != "" {
				_, _ = io.WriteString(w, cfg.infoBody)
				return
			}
			urlPrivate := srv.URL + "/files/" + testFileID
			if cfg.noURL {
				urlPrivate = ""
			}
			_, _ = fmt.Fprintf(w,
				`{"ok":true,"file":{"id":%q,"name":"report.pdf","mimetype":"application/pdf","size":%d,"url_private":%q}}`,
				testFileID, len(cfg.content), urlPrivate)
		case "/files/" + testFileID:
			rec.recordBytes(r.Header.Get("Authorization"))
			if cfg.redirectTo != "" {
				http.Redirect(w, r, cfg.redirectTo, http.StatusFound)
				return
			}
			if cfg.contentStatus == http.StatusTooManyRequests {
				w.Header().Set("Retry-After", cfg.retryAfter)
				w.WriteHeader(cfg.contentStatus)
				return
			}
			if cfg.contentStatus != 0 && cfg.contentStatus != http.StatusOK {
				http.Error(w, "boom", cfg.contentStatus)
				return
			}
			_, _ = w.Write(cfg.content)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	// The generated url_private points at this server (plain http on a
	// loopback port), which the production allowlist rejects — stub it to
	// permit this host for the test's lifetime. keepURLPolicy tests exercise
	// the production policy instead.
	if !cfg.keepURLPolicy {
		u, err := url.Parse(srv.URL)
		require.NoError(t, err)
		saved := validateFileURL
		validateFileURL = func(v *url.URL) bool { return v.Host == u.Host }
		t.Cleanup(func() { validateFileURL = saved })
	}
	return srv, rec
}

// newFileService binds the download endpoint to a client that stamps the
// bearer token, mirroring how dialSlack hands the strategy's client to
// newHTTPService.
func newFileService(t *testing.T, srv *httptest.Server) *httpService {
	t.Helper()
	client := srv.Client()
	client.Transport = &authTransport{base: client.Transport, token: testToken}
	return newHTTPService(client, srv.URL)
}

// readAll reads one file out of an in-memory FS.
func readAll(t *testing.T, fs afero.Fs, path string) []byte {
	t.Helper()
	data, err := afero.ReadFile(fs, path)
	require.NoError(t, err)
	return data
}

// TestFileInfoMapsSharedView pins the files.info mapping: the shared File view
// carries exactly id/name/mimetype/size and the request carries the file id.
// url_private is wire-only and must not survive into the view's JSON.
func TestFileInfoMapsSharedView(t *testing.T) {
	srv, rec := newFileServer(t, fileServerConfig{content: []byte("bytes")})
	svc := newFileService(t, srv)

	file, err := svc.FileInfo(context.Background(), testFileID)
	require.NoError(t, err)
	assert.Equal(t, File{ID: testFileID, Name: "report.pdf", Mimetype: "application/pdf", Size: 5}, file)

	encoded, err := json.Marshal(file)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "url_private")

	queries := rec.infoQueries
	require.Len(t, queries, 1)
	assert.Equal(t, testFileID, queries[0].Get("file"))
}

// TestFileDownloadStdoutVerbatim: download streams the bytes to stdout
// byte-for-byte (control bytes included) and carries the authenticated
// client's Authorization header to the absolute url_private.
func TestFileDownloadStdoutVerbatim(t *testing.T) {
	content := []byte("a\x00b\x07c\xff")
	srv, rec := newFileServer(t, fileServerConfig{content: content})
	_, root, out := newSlackEnv(t)
	stubDial(t, newFileService(t, srv))

	stdout, err := execute(t, root, out, "slack", "file", "download", testFileID)
	require.NoError(t, err)

	assert.Equal(t, content, []byte(stdout))
	assert.Equal(t, "Bearer "+testToken, rec.authHeader())
	assert.Equal(t, 1, rec.bytesHits)
}

// TestFileDownloadOutWritesBytes: --out streams the bytes through the afero
// FS, creating parent directories, and stdout stays empty.
func TestFileDownloadOutWritesBytes(t *testing.T) {
	content := []byte{0x00, 0x01, 0xFF, 0x25}
	srv, rec := newFileServer(t, fileServerConfig{content: content})
	cfg, root, out := newSlackEnv(t)
	stubDial(t, newFileService(t, srv))

	stdout, err := execute(t, root, out, "slack", "file", "download", testFileID, "--out", "dl/nested/report.pdf")
	require.NoError(t, err)

	assert.Empty(t, stdout)
	assert.Equal(t, content, readAll(t, cfg.Fs, "dl/nested/report.pdf"))
	assert.Equal(t, "Bearer "+testToken, rec.authHeader())
}

// TestFileDownloadInfoOKFalseCarriesCode: a files.info HTTP-200 ok:false
// answer surfaces the raw Slack error code unchanged.
func TestFileDownloadInfoOKFalseCarriesCode(t *testing.T) {
	srv, _ := newFileServer(t, fileServerConfig{infoBody: `{"ok":false,"error":"file_not_found"}`})
	_, root, out := newSlackEnv(t)
	stubDial(t, newFileService(t, srv))

	_, err := execute(t, root, out, "slack", "file", "download", testFileID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slack API error: file_not_found")
}

// TestFileDownloadNon200: a non-200 bytes response becomes the shared
// status + capped body error.
func TestFileDownloadNon200(t *testing.T) {
	srv, _ := newFileServer(t, fileServerConfig{contentStatus: http.StatusBadGateway})
	_, root, out := newSlackEnv(t)
	stubDial(t, newFileService(t, srv))

	_, err := execute(t, root, out, "slack", "file", "download", testFileID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slack API returned 502")
	assert.Contains(t, err.Error(), "boom")
}

// TestFileDownloadRateLimited: a 429 on the bytes response maps to the shared
// rate-limit error naming the Retry-After seconds.
func TestFileDownloadRateLimited(t *testing.T) {
	srv, _ := newFileServer(t, fileServerConfig{
		contentStatus: http.StatusTooManyRequests,
		retryAfter:    "42",
	})
	_, root, out := newSlackEnv(t)
	stubDial(t, newFileService(t, srv))

	_, err := execute(t, root, out, "slack", "file", "download", testFileID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retry after 42s")
}

// fileInfoBody renders an ok:true files.info body with a caller-chosen
// url_private, for tests that must control the download URL itself.
func fileInfoBody(urlPrivate string) string {
	return fmt.Sprintf(
		`{"ok":true,"file":{"id":%q,"name":"report.pdf","mimetype":"application/pdf","size":5,"url_private":%q}}`,
		testFileID, urlPrivate)
}

// newAuthRecorder serves 200s and records the Authorization header of every
// request it receives, so tests can prove a refused fetch never reached the
// host — with or without the token.
type authRecorder struct {
	mu    sync.Mutex
	auths []string
}

func newAuthRecorder(t *testing.T) (*httptest.Server, *authRecorder) {
	t.Helper()
	rec := &authRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		rec.auths = append(rec.auths, r.Header.Get("Authorization"))
		rec.mu.Unlock()
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func (r *authRecorder) requests() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.auths...)
}

// TestFileDownloadRejectsUntrustedHost: a url_private on a host outside the
// allowlist fails before any request — the untrusted host never sees a byte.
func TestFileDownloadRejectsUntrustedHost(t *testing.T) {
	evil, evilRec := newAuthRecorder(t)
	// The fixture's stub permits only the info server's host, so the evil
	// host fails the allowlist even before the scheme is considered.
	srv, rec := newFileServer(t, fileServerConfig{infoBody: fileInfoBody(evil.URL + "/files/" + testFileID)})
	_, root, out := newSlackEnv(t)
	stubDial(t, newFileService(t, srv))

	_, err := execute(t, root, out, "slack", "file", "download", testFileID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), testFileID)
	assert.NotContains(t, err.Error(), evil.URL)
	assert.Empty(t, evilRec.requests())
	assert.Equal(t, 0, rec.bytesHits)
}

// TestFileDownloadRejectsHTTPURL: a url_private without the https scheme is
// rejected under the production allowlist, naming the file id and never the
// URL.
func TestFileDownloadRejectsHTTPURL(t *testing.T) {
	const urlPrivate = "http://files.slack.com/files-pri/x/report.pdf"
	srv, rec := newFileServer(t, fileServerConfig{
		infoBody:      fileInfoBody(urlPrivate),
		keepURLPolicy: true,
	})
	_, root, out := newSlackEnv(t)
	stubDial(t, newFileService(t, srv))

	_, err := execute(t, root, out, "slack", "file", "download", testFileID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), testFileID)
	assert.NotContains(t, err.Error(), urlPrivate)
	assert.Equal(t, 0, rec.bytesHits)
}

// TestFileDownloadRefusesCrossHostRedirect: a valid download host 302ing to
// another host is refused, and the redirect target never receives a request
// carrying the bearer token (the transport would re-stamp it after Go's
// client strips it).
func TestFileDownloadRefusesCrossHostRedirect(t *testing.T) {
	evil, evilRec := newAuthRecorder(t)
	srv, _ := newFileServer(t, fileServerConfig{redirectTo: evil.URL + "/stolen"})
	_, root, out := newSlackEnv(t)
	stubDial(t, newFileService(t, srv))

	_, err := execute(t, root, out, "slack", "file", "download", testFileID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cross-host redirect")
	assert.NotContains(t, err.Error(), evil.URL)
	for _, auth := range evilRec.requests() {
		assert.NotContains(t, auth, testToken)
	}
	assert.Empty(t, evilRec.requests())
}

// TestFileDownloadErrorHidesURL: a connection failure surfaces the cause but
// never the url_private string (url.Error embeds it; the error is unwrapped).
func TestFileDownloadErrorHidesURL(t *testing.T) {
	dead, _ := newAuthRecorder(t)
	urlPrivate := dead.URL + "/files/" + testFileID
	dead.Close()

	// Permit the (now dead) loopback host so the failure is a dial error,
	// not an allowlist rejection.
	saved := validateFileURL
	validateFileURL = func(*url.URL) bool { return true }
	t.Cleanup(func() { validateFileURL = saved })

	srv, _ := newFileServer(t, fileServerConfig{infoBody: fileInfoBody(urlPrivate)})
	_, root, out := newSlackEnv(t)
	stubDial(t, newFileService(t, srv))

	_, err := execute(t, root, out, "slack", "file", "download", testFileID)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), urlPrivate)
}

// TestFileDownloadRequiresExactlyOneArg: the file-id positional is mandatory.
func TestFileDownloadRequiresExactlyOneArg(t *testing.T) {
	_, root, out := newSlackEnv(t)

	_, err := execute(t, root, out, "slack", "file", "download")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}
