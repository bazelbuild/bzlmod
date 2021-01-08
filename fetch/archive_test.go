package fetch

import (
	"github.com/bazelbuild/bzlmod/common"
	"github.com/bazelbuild/bzlmod/common/integrity"
	"github.com/bazelbuild/bzlmod/common/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestArchive_SharedRepoDirReady(t *testing.T) {
	TestBzlmodDir = t.TempDir()
	defer func() { TestBzlmodDir = "" }()
	server := testutil.StaticHttpServer(map[string][]byte{}) // deliberately don't serve the "a.zip" that we need
	defer server.Close()
	a := Archive{
		URLs:      []string{server.URL + "/a.zip"},
		Integrity: "",
		Fprint:    "some_fingerprint",
	}

	// If the shared repo dir is ready, we should be happy and not even attempt the download.
	testutil.WriteFile(t, filepath.Join(TestBzlmodDir, "shared_repos", "some_fingerprint", "bzlmod.fingerprint"), "some_fingerprint")

	fp, err := a.Fetch("")
	if assert.NoError(t, err) {
		assert.Equal(t, filepath.Join(TestBzlmodDir, "shared_repos", "some_fingerprint"), fp)
	}
}

func TestArchive_BadFingerprintInSharedRepoDir(t *testing.T) {
	TestBzlmodDir = t.TempDir()
	defer func() { TestBzlmodDir = "" }()

	zipArchive := testutil.BuildZipArchive(t, map[string][]byte{
		"file1":     []byte(`file1contents`),
		"dir/file2": []byte(`file2contents`),
	})
	server := testutil.StaticHttpServer(map[string][]byte{
		"/a.zip": zipArchive,
	})
	defer server.Close()

	a := Archive{
		URLs:      []string{server.URL + "/a.zip"},
		Integrity: integrity.MustGenerate("sha256", zipArchive),
		Fprint:    "some_fingerprint",
	}

	testutil.WriteFile(t, filepath.Join(TestBzlmodDir, "shared_repos", "some_fingerprint", "bzlmod.fingerprint"), "bad_fingerprint")
	testutil.WriteFile(t, filepath.Join(TestBzlmodDir, "shared_repos", "some_fingerprint", "random_file"), "kek")

	fp, err := a.Fetch("")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(TestBzlmodDir, "shared_repos", "some_fingerprint"), fp)
	testutil.AssertFileContents(t, filepath.Join(fp, "bzlmod.fingerprint"), "some_fingerprint")
	testutil.AssertFileContents(t, filepath.Join(fp, "file1"), "file1contents")
	testutil.AssertFileContents(t, filepath.Join(fp, "dir", "file2"), "file2contents")
	// "random_file" should have been deleted.
	_, err = os.Stat(filepath.Join(fp, "random_file"))
	if !assert.True(t, os.IsNotExist(err)) {
		t.Logf("expected NotExist, got: %v", err)
	}
}

func TestArchive_GoodContentsInHTTPCache(t *testing.T) {
	TestBzlmodDir = t.TempDir()
	defer func() { TestBzlmodDir = "" }()
	server := testutil.StaticHttpServer(map[string][]byte{}) // deliberately don't serve the "a.zip" that we need
	defer server.Close()

	zipArchive := testutil.BuildZipArchive(t, map[string][]byte{
		"file1":     []byte(`file1contents`),
		"dir/file2": []byte(`file2contents`),
	})

	a := Archive{
		URLs:      []string{server.URL + "/a.zip"},
		Integrity: integrity.MustGenerate("sha256", zipArchive),
		Fprint:    "some_fingerprint",
	}

	testutil.WriteFileBytes(t, filepath.Join(TestBzlmodDir, "http_cache", common.Hash(server.URL+"/a.zip")), zipArchive)

	fp, err := a.Fetch("")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(TestBzlmodDir, "shared_repos", "some_fingerprint"), fp)
	testutil.AssertFileContents(t, filepath.Join(fp, "bzlmod.fingerprint"), "some_fingerprint")
	testutil.AssertFileContents(t, filepath.Join(fp, "file1"), "file1contents")
	testutil.AssertFileContents(t, filepath.Join(fp, "dir", "file2"), "file2contents")
}

func TestArchive_BadContentsInHTTPCache(t *testing.T) {
	TestBzlmodDir = t.TempDir()
	defer func() { TestBzlmodDir = "" }()

	zipArchive := testutil.BuildZipArchive(t, map[string][]byte{
		"file1":     []byte(`file1contents`),
		"dir/file2": []byte(`file2contents`),
	})
	server := testutil.StaticHttpServer(map[string][]byte{
		"/a.zip": zipArchive,
	})
	defer server.Close()

	a := Archive{
		URLs:      []string{server.URL + "/a.zip"},
		Integrity: integrity.MustGenerate("sha256", zipArchive),
		Fprint:    "some_fingerprint",
	}

	testutil.WriteFile(t, filepath.Join(TestBzlmodDir, "http_cache", common.Hash(server.URL+"/a.zip")),
		"wrong file contents which should fail integrity check")

	fp, err := a.Fetch("")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(TestBzlmodDir, "shared_repos", "some_fingerprint"), fp)
	testutil.AssertFileContents(t, filepath.Join(fp, "bzlmod.fingerprint"), "some_fingerprint")
	testutil.AssertFileContents(t, filepath.Join(fp, "file1"), "file1contents")
	testutil.AssertFileContents(t, filepath.Join(fp, "dir", "file2"), "file2contents")
}

func TestArchive_DownloadCascade(t *testing.T) {
	TestBzlmodDir = t.TempDir()
	defer func() { TestBzlmodDir = "" }()

	zipArchive := testutil.BuildZipArchive(t, map[string][]byte{
		"file1":     []byte(`file1contents`),
		"dir/file2": []byte(`file2contents`),
	})
	anotherZipArchive := testutil.BuildZipArchive(t, map[string][]byte{
		"file3": []byte(`file3contents`),
	})
	server := testutil.StaticHttpServer(map[string][]byte{
		"/bad.zip":          []byte(`whatever`),
		"/good.zip":         zipArchive,
		"/another/good.zip": anotherZipArchive,
	})
	defer server.Close()

	a := Archive{
		URLs: []string{
			server.URL + "/bad.zip",          // fails integrity check
			server.URL + "/nonexistent.zip",  // 404
			server.URL + "/good.zip",         // good! chosen
			server.URL + "/another/good.zip", // also good (also passes integrity), but test that this is _not_ used
		},
		Integrity: integrity.MustGenerate("sha256", zipArchive) + " " + integrity.MustGenerate("sha256", anotherZipArchive),
		Fprint:    "some_fingerprint",
	}

	fp, err := a.Fetch("")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(TestBzlmodDir, "shared_repos", "some_fingerprint"), fp)
	testutil.AssertFileContents(t, filepath.Join(fp, "bzlmod.fingerprint"), "some_fingerprint")
	testutil.AssertFileContents(t, filepath.Join(fp, "file1"), "file1contents")
	testutil.AssertFileContents(t, filepath.Join(fp, "dir", "file2"), "file2contents")

	testutil.AssertFileContentsBytes(t, filepath.Join(TestBzlmodDir, "http_cache", common.Hash(server.URL+"/good.zip")), zipArchive)
}

func TestArchive_DownloadFails(t *testing.T) {
	TestBzlmodDir = t.TempDir()
	defer func() { TestBzlmodDir = "" }()

	zipArchive := testutil.BuildZipArchive(t, map[string][]byte{
		"file1":     []byte(`file1contents`),
		"dir/file2": []byte(`file2contents`),
	})
	server := testutil.StaticHttpServer(map[string][]byte{
		"/a.zip": zipArchive,
	})
	defer server.Close()

	a := Archive{
		URLs: []string{
			server.URL + "/a.zip",           // fails integrity check
			server.URL + "/nonexistent.zip", // 404
			"gopher://something",            // unrecognized scheme
		},
		Integrity: integrity.MustGenerate("sha256", []byte(`fail the integrity check!`)),
		Fprint:    "some_fingerprint",
	}

	_, err := a.Fetch("")
	require.Error(t, err)
}

func TestArchive_FileScheme(t *testing.T) {
	tempDir := t.TempDir()
	TestBzlmodDir = filepath.Join(tempDir, "bzlmod")
	defer func() { TestBzlmodDir = "" }()

	zipArchive := testutil.BuildZipArchive(t, map[string][]byte{
		"file1":     []byte(`file1contents`),
		"dir/file2": []byte(`file2contents`),
	})
	testutil.WriteFileBytes(t, filepath.Join(tempDir, "good.zip"), zipArchive)
	testutil.WriteFile(t, filepath.Join(tempDir, "bad.zip"), "random stuff")

	a := Archive{
		URLs: []string{
			"file://" + filepath.ToSlash(filepath.Join(tempDir, "bad.zip")),         // fails integrity check
			"file://" + filepath.ToSlash(filepath.Join(tempDir, "nonexistent.zip")), // nonexistent
			"file://" + filepath.ToSlash(filepath.Join(tempDir, "good.zip")),
		},
		Integrity: integrity.MustGenerate("sha256", zipArchive),
		Fprint:    "some_fingerprint",
	}

	fp, err := a.Fetch("")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(TestBzlmodDir, "shared_repos", "some_fingerprint"), fp)
	testutil.AssertFileContents(t, filepath.Join(fp, "bzlmod.fingerprint"), "some_fingerprint")
	testutil.AssertFileContents(t, filepath.Join(fp, "file1"), "file1contents")
	testutil.AssertFileContents(t, filepath.Join(fp, "dir", "file2"), "file2contents")
}

func TestArchive_Vendor_VendorDirReady(t *testing.T) {
	tempDir := t.TempDir()
	TestBzlmodDir = filepath.Join(tempDir, "bzlmod")
	defer func() { TestBzlmodDir = "" }()
	server := testutil.StaticHttpServer(map[string][]byte{}) // deliberately don't serve the "a.zip" that we need
	defer server.Close()
	a := Archive{
		URLs:      []string{server.URL + "/a.zip"},
		Integrity: "",
		Fprint:    "some_fingerprint",
	}

	// Create a vendor dir with a matching fingerprint file.
	vendorDir := filepath.Join(tempDir, "vendor")
	testutil.WriteFile(t, filepath.Join(vendorDir, "bzlmod.fingerprint"), "some_fingerprint")

	// This should be a no-op.
	fp, err := a.Fetch(vendorDir)
	require.NoError(t, err)
	require.Equal(t, vendorDir, fp)
}

func TestArchive_Vendor_BadFingerprint(t *testing.T) {
	tempDir := t.TempDir()
	TestBzlmodDir = filepath.Join(tempDir, "bzlmod")
	defer func() { TestBzlmodDir = "" }()

	zipArchive := testutil.BuildZipArchive(t, map[string][]byte{
		"file1":     []byte(`file1contents`),
		"dir/file2": []byte(`file2contents`),
	})
	server := testutil.StaticHttpServer(map[string][]byte{
		"/a.zip": zipArchive,
	})
	defer server.Close()
	a := Archive{
		URLs:      []string{server.URL + "/a.zip"},
		Integrity: "",
		Fprint:    "some_fingerprint",
	}

	// Create a vendor dir with a bad fingerprint file.
	vendorDir := filepath.Join(tempDir, "vendor")
	testutil.WriteFile(t, filepath.Join(vendorDir, "bzlmod.fingerprint"), "oopsie daisie")
	testutil.WriteFile(t, filepath.Join(vendorDir, "random_file"), "something")

	// We should still fetch it via HTTP.
	fp, err := a.Fetch(vendorDir)
	require.NoError(t, err)
	require.Equal(t, vendorDir, fp)
	testutil.AssertFileContents(t, filepath.Join(fp, "bzlmod.fingerprint"), "some_fingerprint")
	testutil.AssertFileContents(t, filepath.Join(fp, "file1"), "file1contents")
	testutil.AssertFileContents(t, filepath.Join(fp, "dir", "file2"), "file2contents")
	// "random_file" should have been deleted.
	_, err = os.Stat(filepath.Join(fp, "random_file"))
	if !assert.True(t, os.IsNotExist(err)) {
		t.Logf("expected NotExist, got: %v", err)
	}
}

func TestArchive_Vendor_NoFingerprintFile(t *testing.T) {
	tempDir := t.TempDir()
	TestBzlmodDir = filepath.Join(tempDir, "bzlmod")
	defer func() { TestBzlmodDir = "" }()

	zipArchive := testutil.BuildZipArchive(t, map[string][]byte{
		"file1":     []byte(`file1contents`),
		"dir/file2": []byte(`file2contents`),
	})
	server := testutil.StaticHttpServer(map[string][]byte{
		"/a.zip": zipArchive,
	})
	defer server.Close()
	a := Archive{
		URLs:      []string{server.URL + "/a.zip"},
		Integrity: "",
		Fprint:    "some_fingerprint",
	}

	// Create a vendor dir without a fingerprint file.
	vendorDir := filepath.Join(tempDir, "vendor")
	testutil.WriteFile(t, filepath.Join(vendorDir, "random_file"), "something")

	// We should still fetch it via HTTP.
	fp, err := a.Fetch(vendorDir)
	require.NoError(t, err)
	require.Equal(t, vendorDir, fp)
	testutil.AssertFileContents(t, filepath.Join(fp, "bzlmod.fingerprint"), "some_fingerprint")
	testutil.AssertFileContents(t, filepath.Join(fp, "file1"), "file1contents")
	testutil.AssertFileContents(t, filepath.Join(fp, "dir", "file2"), "file2contents")
	// "random_file" should have been deleted.
	_, err = os.Stat(filepath.Join(fp, "random_file"))
	if !assert.True(t, os.IsNotExist(err)) {
		t.Logf("expected NotExist, got: %v", err)
	}
}

func TestArchive_Vendor_CopyFromSharedRepoDir(t *testing.T) {
	tempDir := t.TempDir()
	TestBzlmodDir = filepath.Join(tempDir, "bzlmod")
	defer func() { TestBzlmodDir = "" }()
	server := testutil.StaticHttpServer(map[string][]byte{}) // deliberately don't serve the "a.zip" that we need
	defer server.Close()
	a := Archive{
		URLs:      []string{server.URL + "/a.zip"},
		Integrity: "",
		Fprint:    "some_fingerprint",
	}

	// The vendor dir doesn't exist at all. But the shared repo dir is ready, so we should be happy to use that, and
	// copy everything over.
	vendorDir := filepath.Join(tempDir, "vendor")
	testutil.WriteFile(t, filepath.Join(TestBzlmodDir, "shared_repos", "some_fingerprint", "bzlmod.fingerprint"), "some_fingerprint")
	testutil.WriteFile(t, filepath.Join(TestBzlmodDir, "shared_repos", "some_fingerprint", "file1"), "file1contents")
	testutil.WriteFile(t, filepath.Join(TestBzlmodDir, "shared_repos", "some_fingerprint", "dir", "file2"), "file2contents")

	fp, err := a.Fetch(vendorDir)
	require.NoError(t, err)
	require.Equal(t, vendorDir, fp)
	testutil.AssertFileContents(t, filepath.Join(fp, "bzlmod.fingerprint"), "some_fingerprint")
	testutil.AssertFileContents(t, filepath.Join(fp, "file1"), "file1contents")
	testutil.AssertFileContents(t, filepath.Join(fp, "dir", "file2"), "file2contents")
}

// TODO: test StripPrefix
