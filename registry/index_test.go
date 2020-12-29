package registry

import (
	"errors"
	"github.com/bazelbuild/bzlmod/fetch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, filename string, contents string) {
	if err := os.MkdirAll(filepath.Dir(filename), 0777); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filename, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestFileIndex_URL(t *testing.T) {
	dir := t.TempDir()
	rawurl := "file://" + filepath.ToSlash(dir)
	fi, err := New(rawurl)
	require.NoError(t, err)
	assert.Equal(t, rawurl, fi.URL())
}

func TestFileIndex_GetModuleBazel(t *testing.T) {
	dir := t.TempDir()
	fi, err := New("file://" + filepath.ToSlash(dir))
	require.NoError(t, err)

	writeFile(t, filepath.Join(dir, "A", "1.0", "MODULE.bazel"), "kek")
	writeFile(t, filepath.Join(dir, "B", "2.0", "MODULE.bazel"), "lel")

	bytes, err := fi.GetModuleBazel("A", "1.0")
	if assert.NoError(t, err) {
		assert.Equal(t, []byte("kek"), bytes)
	}

	bytes, err = fi.GetModuleBazel("B", "2.0")
	if assert.NoError(t, err) {
		assert.Equal(t, []byte("lel"), bytes)
	}

	bytes, err = fi.GetModuleBazel("A", "2.0")
	if err == nil {
		t.Errorf("unexpected success getting A@2.0: got %v", string(bytes))
	} else {
		assert.True(t, errors.Is(err, ErrNotFound))
	}
}

func TestFileIndex_GetFetcher(t *testing.T) {
	dir := t.TempDir()
	fi, err := New("file://" + filepath.ToSlash(dir))
	require.NoError(t, err)

	writeFile(t, filepath.Join(dir, "bazel_registry.json"), `{
  "mirrors": [
    "https://mirror.bazel.build/",
    "file:///home/bazel/mymirror/"
  ]
}`)
	writeFile(t, filepath.Join(dir, "A", "1.0", "source.json"), `{
  "url": "http://mysite.com/thing.zip",
  "integrity": "sha256-blah",
  "strip_prefix": "pref"
}`)
	writeFile(t, filepath.Join(dir, "A", "2.0", "source.json"), `{
  "url": "https://github.com/lol.tar.gz",
  "integrity": "sha256-bleh",
  "patch_files": ["1-fix-this.patch", "2-fix-that.patch"]
}`)
	writeFile(t, filepath.Join(dir, "B", "1.0", "source.json"), `{
  "url": "https://example.com/archive.jar?with=query",
  "integrity": "sha256-bluh"
}`)

	fetcher, err := fi.GetFetcher("A", "1.0")
	if assert.NoError(t, err) {
		assert.Equal(t, &fetch.Archive{
			URLs: []string{
				"https://mirror.bazel.build/mysite.com/thing.zip",
				"file:///home/bazel/mymirror/mysite.com/thing.zip",
				"http://mysite.com/thing.zip",
			},
			Integrity:   "sha256-blah",
			StripPrefix: "pref",
		}, fetcher)
	}

	fetcher, err = fi.GetFetcher("A", "2.0")
	if assert.NoError(t, err) {
		assert.Equal(t, &fetch.Archive{
			URLs: []string{
				"https://mirror.bazel.build/github.com/lol.tar.gz",
				"file:///home/bazel/mymirror/github.com/lol.tar.gz",
				"https://github.com/lol.tar.gz",
			},
			Integrity: "sha256-bleh",
			PatchFiles: []string{
				fi.URL() + "/A/2.0/patches/1-fix-this.patch",
				fi.URL() + "/A/2.0/patches/2-fix-that.patch",
			},
		}, fetcher)
	}

	fetcher, err = fi.GetFetcher("B", "1.0")
	if assert.NoError(t, err) {
		assert.Equal(t, &fetch.Archive{
			URLs: []string{
				"https://mirror.bazel.build/example.com/archive.jar?with=query",
				"file:///home/bazel/mymirror/example.com/archive.jar?with=query",
				"https://example.com/archive.jar?with=query",
			},
			Integrity: "sha256-bluh",
		}, fetcher)
	}
}
