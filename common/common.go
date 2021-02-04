package common

import (
	"fmt"
	"path/filepath"
	"strings"
)

type ModuleKey struct {
	Name    string
	Version string // empty for modules with LocalPath/URL/Git overrides
}

func (k *ModuleKey) String() string {
	if k.Version == "" {
		return fmt.Sprintf("%v@_", k.Name)
	}
	return fmt.Sprintf("%v@%v", k.Name, k.Version)
}

// NormalizePath normalizes `path`, which can be either absolute or relative to `root`, to an absolute file path. If
// `path` is an absolute path on the current OS, we just return it; otherwise, it could either have forward slashes or
// backward slashes as path separators, and we deal with it accordingly. `root` itself should already be an absolute
// filepath.
func NormalizePath(root string, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if strings.IndexByte(path, '/') >= 0 {
		path = filepath.FromSlash(path)
	}
	return filepath.Join(root, path)
}
