package fetch

import (
	"github.com/bazelbuild/bzlmod/common"
	"os"
	"path/filepath"
)

// TestBzlmodDir can be set in order to override the normal bzlmod cache dir for testing. Usage:
//   func TestSomething(t *testing.T) {
//     fetch.TestBzlmodDir = t.TempDir()
//     defer func() { fetch.TestBzlmodDir = "" }()
//     ...
//   }
var TestBzlmodDir = ""

func BzlmodDir() (string, error) {
	if TestBzlmodDir != "" {
		return TestBzlmodDir, nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "bzlmod"), nil
}

// SharedRepoDir returns the path to the directory under which the shared repo identified by the given hash is placed.
func SharedRepoDir(hash string) (string, error) {
	bzlmodDir, err := BzlmodDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(bzlmodDir, "shared_repos", hash), nil
}

func HTTPCacheFilePath(url string) (string, error) {
	bzlmodDir, err := BzlmodDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(bzlmodDir, "http_cache", common.Hash(url)), nil
}

// BzlmodWsDir returns the path to the workspace-specific directory for `wsDir`.
func BzlmodWsDir(wsDir string) (string, error) {
	bzlmodDir, err := BzlmodDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(bzlmodDir, "ws", common.Hash(wsDir)), nil
}
