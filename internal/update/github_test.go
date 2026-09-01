package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testToken = "test-token-value"

func testServer(t *testing.T, status int, body string) (*httptest.Server, *[]*http.Request) {
	t.Helper()
	var got []*http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func TestLatestRelease(t *testing.T) {
	ctx := context.Background()

	t.Run("parses json", func(t *testing.T) {
		srv, _ := testServer(t, http.StatusOK,
			`{"tag_name":"v1.2.3","assets":[{"name":"everything-cli_darwin_arm64.tar.gz","browser_download_url":"http://dl/darwin.tar.gz"}]}`)
		c := NewClient(srv.URL, "owner/repo")
		rel, err := c.LatestRelease(ctx)
		require.NoError(t, err)
		assert.Equal(t, "v1.2.3", rel.Tag)
		require.Len(t, rel.Assets, 1)
		assert.Equal(t, "everything-cli_darwin_arm64.tar.gz", rel.Assets[0].Name)
		assert.Equal(t, "http://dl/darwin.tar.gz", rel.Assets[0].URL)
	})

	t.Run("404 is ErrNoReleases", func(t *testing.T) {
		srv, _ := testServer(t, http.StatusNotFound, `{}`)
		_, err := NewClient(srv.URL, "owner/repo").LatestRelease(ctx)
		require.ErrorIs(t, err, ErrNoReleases)
	})

	t.Run("403 is ErrRateLimited", func(t *testing.T) {
		srv, _ := testServer(t, http.StatusForbidden, `{}`)
		_, err := NewClient(srv.URL, "owner/repo").LatestRelease(ctx)
		require.ErrorIs(t, err, ErrRateLimited)
	})

	t.Run("other status is plain error", func(t *testing.T) {
		srv, _ := testServer(t, http.StatusInternalServerError, "")
		_, err := NewClient(srv.URL, "owner/repo").LatestRelease(ctx)
		require.Error(t, err)
		assert.False(t, errors.Is(err, ErrNoReleases))
		assert.False(t, errors.Is(err, ErrRateLimited))
	})
}

func TestLatestReleaseHeaders(t *testing.T) {
	ctx := context.Background()

	t.Run("accept header sent", func(t *testing.T) {
		srv, reqs := testServer(t, http.StatusOK, `{"tag_name":"v1.0.0","assets":[]}`)
		_, err := NewClient(srv.URL, "owner/repo").LatestRelease(ctx)
		require.NoError(t, err)
		require.Len(t, *reqs, 1)
		assert.Equal(t, "application/vnd.github+json", (*reqs)[0].Header.Get("Accept"))
		assert.Equal(t, "/repos/owner/repo/releases/latest", (*reqs)[0].URL.Path)
	})

	t.Run("auth header when GITHUB_TOKEN set", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", testToken)
		srv, reqs := testServer(t, http.StatusOK, `{"tag_name":"v1.0.0","assets":[]}`)
		_, err := NewClient(srv.URL, "owner/repo").LatestRelease(ctx)
		require.NoError(t, err)
		require.Len(t, *reqs, 1)
		assert.Equal(t, "Bearer "+testToken, (*reqs)[0].Header.Get("Authorization"))
	})

	t.Run("auth header from GH_TOKEN", func(t *testing.T) {
		t.Setenv("GH_TOKEN", testToken)
		srv, reqs := testServer(t, http.StatusOK, `{"tag_name":"v1.0.0","assets":[]}`)
		_, err := NewClient(srv.URL, "owner/repo").LatestRelease(ctx)
		require.NoError(t, err)
		require.Len(t, *reqs, 1)
		assert.Equal(t, "Bearer "+testToken, (*reqs)[0].Header.Get("Authorization"))
	})

	t.Run("no auth header without env", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("GH_TOKEN", "")
		srv, reqs := testServer(t, http.StatusOK, `{"tag_name":"v1.0.0","assets":[]}`)
		_, err := NewClient(srv.URL, "owner/repo").LatestRelease(ctx)
		require.NoError(t, err)
		require.Len(t, *reqs, 1)
		_, has := (*reqs)[0].Header["Authorization"]
		assert.False(t, has)
	})
}

func TestDownload(t *testing.T) {
	ctx := context.Background()
	payload := []byte("tar.gz bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "owner/repo")
	got, err := c.Download(ctx, srv.URL+"/download/everything-cli_darwin_arm64.tar.gz")
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

func TestResponseSizeCap(t *testing.T) {
	ctx := context.Background()

	orig := maxBodyLimit
	t.Cleanup(func() { maxBodyLimit = orig })

	t.Run("oversized body is rejected with host and limit", func(t *testing.T) {
		maxBodyLimit = 8
		srv, _ := testServer(t, http.StatusOK, "0123456789")

		_, err := NewClient(srv.URL, "owner/repo").LatestRelease(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds 8 bytes")
		assert.Contains(t, err.Error(), srv.Listener.Addr().String())
	})

	t.Run("cap applies to Download too", func(t *testing.T) {
		maxBodyLimit = 4
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("0123456789"))
		}))
		t.Cleanup(srv.Close)

		_, err := NewClient(srv.URL, "owner/repo").Download(ctx, srv.URL+"/asset.tar.gz")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds 4 bytes")
	})

	t.Run("body exactly at cap is read fully", func(t *testing.T) {
		maxBodyLimit = 12
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("tar.gz bytes"))
		}))
		t.Cleanup(srv.Close)

		got, err := NewClient(srv.URL, "owner/repo").Download(ctx, srv.URL+"/asset.tar.gz")
		require.NoError(t, err)
		assert.Equal(t, "tar.gz bytes", string(got))
	})
}

func TestReleaseAssetLookup(t *testing.T) {
	rel := &Release{
		Tag: "v1.2.3",
		Assets: []Asset{
			{Name: "everything-cli_darwin_arm64.tar.gz", URL: "http://dl/darwin.tar.gz"},
			{Name: "everything-cli_linux_amd64.tar.gz", URL: "http://dl/linux.tar.gz"},
		},
	}

	tests := []struct {
		name    string
		in      string
		wantErr error
	}{
		{name: "hit", in: "everything-cli_linux_amd64.tar.gz"},
		{name: "miss", in: "everything-cli_windows_386.tar.gz", wantErr: ErrAssetNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := rel.Asset(tt.in)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.in, a.Name)
			assert.Equal(t, rel.Assets[1].URL, a.URL)
		})
	}
}

// TestDefaultRepo pins the release repo slug. It must stay
// "oskarhane/google-cli" — matching scripts/install.sh's REPO — until the
// GitHub repo is renamed; flip it deliberately at that point.
func TestDefaultRepo(t *testing.T) {
	assert.Equal(t, "oskarhane/google-cli", defaultRepo)
}

func TestAssetName(t *testing.T) {
	assert.Equal(t, "everything-cli_darwin_arm64.tar.gz", AssetName("darwin", "arm64"))
	assert.Equal(t, "everything-cli_linux_amd64.tar.gz", AssetName("linux", "amd64"))
}
