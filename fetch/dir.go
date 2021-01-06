package fetch

import (
	"github.com/bazelbuild/bzlmod/common"
	"os"
	"path/filepath"
)

var testBzlmodDir = ""

func BzlmodDir() (string, error) {
	if testBzlmodDir != "" {
		return testBzlmodDir, nil
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
