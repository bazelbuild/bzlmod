package testutil

import (
	"archive/zip"
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"
)

func TestStaticHttpServer(t *testing.T) {
	server := StaticHttpServer(map[string][]byte{
		"/a":     []byte("a"),
		"/b/a":   []byte("ba"),
		"/c/b/a": []byte("cba"),
	})
	defer server.Close()

	resp, err := http.Get(server.URL + "/a")
	if assert.NoError(t, err) {
		b, err := ioutil.ReadAll(resp.Body)
		if assert.NoError(t, err) {
			assert.Equal(t, "a", string(b))
		}
	}

	resp, err = http.Get(server.URL + "/b/a")
	if assert.NoError(t, err) {
		b, err := ioutil.ReadAll(resp.Body)
		if assert.NoError(t, err) {
			assert.Equal(t, "ba", string(b))
		}
	}

	resp, err = http.Get(server.URL + "/c/b/a")
	if assert.NoError(t, err) {
		b, err := ioutil.ReadAll(resp.Body)
		if assert.NoError(t, err) {
			assert.Equal(t, "cba", string(b))
		}
	}

	resp, err = http.Get(server.URL + "/d/c/b/a")
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	}
}

func TestBuildZipArchive(t *testing.T) {
	files := map[string][]byte{
		"a":     []byte("a"),
		"b/a":   []byte("ba"),
		"c/b/a": []byte("cba"),
	}
	a := BuildZipArchive(t, files)
	ar, err := zip.NewReader(bytes.NewReader(a), int64(len(a)))
	require.NoError(t, err)
	for _, f := range ar.File {
		expected, ok := files[f.Name]
		if assert.True(t, ok, f.Name) {
			delete(files, f.Name)
			fr, err := f.Open()
			if assert.NoError(t, err, f.Name) {
				actual, err := ioutil.ReadAll(fr)
				if assert.NoError(t, err, f.Name) {
					assert.Equal(t, string(expected), string(actual))
				}
			}
			assert.NoError(t, fr.Close(), f.Name)
		}
	}
	assert.Empty(t, files)
}

func TestWriteAssertFile(t *testing.T) {
	dir := t.TempDir()
	WriteFile(t, filepath.Join(dir, "a", "b", "c"), "ping pong")
	AssertFileContents(t, filepath.Join(dir, "a", "b", "c"), "ping pong")
	WriteFileBytes(t, filepath.Join(dir, "def", "ghi"), []byte("TABLE TENNIS"))
	AssertFileContentsBytes(t, filepath.Join(dir, "def", "ghi"), []byte("TABLE TENNIS"))
}
