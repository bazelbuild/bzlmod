package registry

import (
	"errors"
	"github.com/bazelbuild/bzlmod/common"
	"github.com/bazelbuild/bzlmod/common/testutil"
	"github.com/bazelbuild/bzlmod/fetch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestIndex_URL(t *testing.T) {
	for _, url := range []string{"file:///home/my/reg", "http://kek.com/", "https://blah.net/something"} {
		i, err := New(url)
		if assert.NoError(t, err, url) {
			assert.Equal(t, url, i.URL())
		}
	}
}

func setUpServerAndLocalFiles(t *testing.T, dir string, files map[string][]byte) *httptest.Server {
	for fname, fbytes := range files {
		testutil.WriteFileBytes(t, filepath.Join(dir, filepath.FromSlash(fname)), fbytes)
	}
	return testutil.StaticHttpServer(files)
}

func TestIndex_GetModuleBazel(t *testing.T) {
	dir := t.TempDir()
	server := setUpServerAndLocalFiles(t, dir, map[string][]byte{
		"/A/1.0/MODULE.bazel": []byte("kek"),
		"/B/2.0/MODULE.bazel": []byte("lel"),
	})
	defer server.Close()

	fi, err := New("file://" + filepath.ToSlash(dir))
	require.NoError(t, err)
	hi, err := New(server.URL)
	require.NoError(t, err)

	for _, reg := range []Registry{fi, hi} {
		bytes, err := reg.GetModuleBazel(common.ModuleKey{"A", "1.0"})
		if assert.NoError(t, err, reg.URL()) {
			assert.Equal(t, []byte("kek"), bytes, reg.URL())
		}

		bytes, err = reg.GetModuleBazel(common.ModuleKey{"B", "2.0"})
		if assert.NoError(t, err, reg.URL()) {
			assert.Equal(t, []byte("lel"), bytes, reg.URL())
		}

		bytes, err = reg.GetModuleBazel(common.ModuleKey{"A", "2.0"})
		if err == nil {
			t.Errorf("unexpected success getting A@2.0 from %v: got %v", reg.URL(), string(bytes))
		} else {
			assert.True(t, errors.Is(err, ErrNotFound), reg.URL())
		}
	}
}

func TestIndex_GetFetcher(t *testing.T) {
	dir := t.TempDir()
	server := setUpServerAndLocalFiles(t, dir, map[string][]byte{
		"/bazel_registry.json": []byte(`{
  "mirrors": [
    "https://mirror.bazel.build/",
    "file:///home/bazel/mymirror/"
  ]
}`),
		"/A/1.0/source.json": []byte(`{
  "url": "http://mysite.com/thing.zip",
  "integrity": "sha256-blah",
  "strip_prefix": "pref"
}`),
		"/A/2.0/source.json": []byte(`{
  "url": "https://github.com/lol.tar.gz",
  "integrity": "sha256-bleh",
  "patch_files": ["1-fix-this.patch", "2-fix-that.patch"]
}`),
		"/B/1.0/source.json": []byte(`{
  "url": "https://example.com/archive.jar?with=query",
  "integrity": "sha256-bluh"
}`),
	})
	defer server.Close()

	fi, err := New("file://" + filepath.ToSlash(dir))
	require.NoError(t, err)
	hi, err := New(server.URL)
	require.NoError(t, err)

	for _, reg := range []Registry{fi, hi} {
		fetcher, err := reg.GetFetcher(common.ModuleKey{"A", "1.0"})
		if assert.NoError(t, err, reg.URL()) {
			assert.Equal(t, &fetch.Archive{
				URLs: []string{
					"https://mirror.bazel.build/mysite.com/thing.zip",
					"file:///home/bazel/mymirror/mysite.com/thing.zip",
					"http://mysite.com/thing.zip",
				},
				Integrity:   "sha256-blah",
				StripPrefix: "pref",
				Fingerprint: common.Hash("regModule", "A", "1.0", reg.URL()),
			}, fetcher, reg.URL())
		}

		fetcher, err = reg.GetFetcher(common.ModuleKey{"A", "2.0"})
		if assert.NoError(t, err, reg.URL()) {
			assert.Equal(t, &fetch.Archive{
				URLs: []string{
					"https://mirror.bazel.build/github.com/lol.tar.gz",
					"file:///home/bazel/mymirror/github.com/lol.tar.gz",
					"https://github.com/lol.tar.gz",
				},
				Integrity: "sha256-bleh",
				PatchFiles: []string{
					reg.URL() + "/A/2.0/patches/1-fix-this.patch",
					reg.URL() + "/A/2.0/patches/2-fix-that.patch",
				},
				Fingerprint: common.Hash("regModule", "A", "2.0", reg.URL()),
			}, fetcher, reg.URL())
		}

		fetcher, err = reg.GetFetcher(common.ModuleKey{"B", "1.0"})
		if assert.NoError(t, err, reg.URL()) {
			assert.Equal(t, &fetch.Archive{
				URLs: []string{
					"https://mirror.bazel.build/example.com/archive.jar?with=query",
					"file:///home/bazel/mymirror/example.com/archive.jar?with=query",
					"https://example.com/archive.jar?with=query",
				},
				Integrity:   "sha256-bluh",
				Fingerprint: common.Hash("regModule", "B", "1.0", reg.URL()),
			}, fetcher, reg.URL())
		}
	}
}
