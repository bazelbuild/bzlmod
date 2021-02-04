package fetch

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
)

type Env struct {
	// VendorDir should be an absolute filepath to the vendor directory.
	VendorDir string
	// WsDir should be an absolute filepath to the root of the workspace directory.
	WsDir         string
	LabelResolver common.LabelResolver
}

// Fetcher contains all the information needed to "fetch" a repo. "Fetch" here is simply defined as making the contents
// of a repo available in a local directory through some means.
type Fetcher interface {
	// Fetch performs the fetch and returns the absolute path to the local directory where the fetched contents can be
	// accessed.
	// Fetch should be idempotent; that is, calling Fetch multiple times in a row should yield the same effect as
	// calling it once. In other words, subsequent calls to Fetch should terminate as early as possible.
	// If vendorDir is non-empty, we're operating in vendoring mode; Fetch should make the contents available under
	// vendorDir if appropriate. Otherwise, Fetch is free to place the contents wherever.
	Fetch(repoName string, env *Env) (string, error)

	// Fingerprint returns a fingerprint of the fetched contents. When the fingerprint changes, it's a signal that the
	// repo should be re-fetched. Note that the fingerprint need not necessarily be calculated from the actual bytes of
	// fetched contents.
	Fingerprint() string

	// AppendPatches appends an extra set of patches to the Fetcher. This can return an error if, for example, this
	// Fetcher doesn't support patches.
	AppendPatches(patches []Patch) error
}

// EarlyFetcher is a Fetcher that can be fetched before all relevant information becomes available (for example, what
// the vendor dir is, what other repos there are, etc). Fetchers that are more "simplistic" and can be used during
// discovery should implement this.
type EarlyFetcher interface {
	Fetcher
	// EarlyFetch is just like Fetch, except that it doesn't get any information about the vendor dir, or the repo name,
	// etc.
	EarlyFetch(wsDir string) (string, error)
}

// LocalPath represents a locally available unpacked directory.
type LocalPath struct {
	Path string
}

func (lp *LocalPath) Fetch(_ string, env *Env) (string, error) {
	// Return the local path as-is, even in vendoring mode.
	return lp.EarlyFetch(env.WsDir)
}

func (lp *LocalPath) Fingerprint() string {
	// The local path never needs to be re-fetched.
	return ""
}

func (lp *LocalPath) AppendPatches(_ []Patch) error {
	return fmt.Errorf("LocalPath fetcher does not support patches")
}

func (lp *LocalPath) EarlyFetch(wsDir string) (string, error) {
	return common.NormalizePath(wsDir, lp.Path), nil
}
