package testutil

import (
	"archive/zip"
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func StaticHttpServer(files map[string][]byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		p, ok := files[req.URL.Path]
		if ok {
			_, _ = w.Write(p)
		} else {
			http.NotFound(w, req)
		}
	}))
}

func BuildZipArchive(t *testing.T, files map[string][]byte) []byte {
	b := &bytes.Buffer{}
	w := zip.NewWriter(b)
	for path, contents := range files {
		fw, err := w.Create(path)
		require.NoError(t, err, path)
		_, err = fw.Write(contents)
		require.NoError(t, err, path)
	}
	require.NoError(t, w.Close())
	return b.Bytes()
}

func WriteFile(t *testing.T, filename string, contents string) {
	WriteFileBytes(t, filename, []byte(contents))
}

func WriteFileBytes(t *testing.T, filename string, contents []byte) {
	require.NoError(t, os.MkdirAll(filepath.Dir(filename), 0777))
	require.NoError(t, ioutil.WriteFile(filename, contents, 0644))
}

func AssertFileContents(t *testing.T, filename string, contents string) {
	actual, err := ioutil.ReadFile(filename)
	if assert.NoError(t, err) {
		assert.Equal(t, contents, string(actual))
	}
}

func AssertFileContentsBytes(t *testing.T, filename string, contents []byte) {
	actual, err := ioutil.ReadFile(filename)
	if assert.NoError(t, err) {
		assert.Equal(t, contents, actual)
	}
}
