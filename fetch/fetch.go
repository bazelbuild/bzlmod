package fetch

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/modrule"
)

type Env struct {
	VendorDir     string
	WsDir         string
	LabelResolver modrule.LabelResolver
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
	EarlyFetch() (string, error)
}

// Wrapper wraps all known implementations of the Fetcher interface and acts as a multiplexer (only 1 member should be
// non-nil). It's useful in JSON marshalling/unmarshalling.
type Wrapper struct {
	Archive   *Archive   `json:",omitempty"`
	Git       *Git       `json:",omitempty"`
	LocalPath *LocalPath `json:",omitempty"`
	ModRule   *ModRule   `json:",omitempty"`
}

func Wrap(f Fetcher) Wrapper {
	switch ft := f.(type) {
	case *Archive:
		return Wrapper{Archive: ft}
	case *Git:
		return Wrapper{Git: ft}
	case *LocalPath:
		return Wrapper{LocalPath: ft}
	case *ModRule:
		return Wrapper{ModRule: ft}
	}
	return Wrapper{}
}

func (w Wrapper) Unwrap() Fetcher {
	if w.Archive != nil {
		return w.Archive
	}
	if w.Git != nil {
		return w.Git
	}
	if w.LocalPath != nil {
		return w.LocalPath
	}
	return w.ModRule
}

func (w Wrapper) Fetch(repoName string, env *Env) (string, error) {
	return w.Unwrap().Fetch(repoName, env)
}

func (w Wrapper) Fingerprint() string {
	return w.Unwrap().Fingerprint()
}

func (w Wrapper) AppendPatches(patches []Patch) error {
	return w.Unwrap().AppendPatches(patches)
}

// LocalPath represents a locally available unpacked directory.
type LocalPath struct {
	Path string
}

func (lp *LocalPath) Fetch(_ string, _ *Env) (string, error) {
	// Return the local path as-is, even in vendoring mode.
	// TODO: filepath.Abs
	return lp.Path, nil
}

func (lp *LocalPath) Fingerprint() string {
	// The local path never needs to be re-fetched.
	return ""
}

func (lp *LocalPath) AppendPatches(_ []Patch) error {
	return fmt.Errorf("LocalPath fetcher does not support patches")
}

func (lp *LocalPath) EarlyFetch() (string, error) {
	return lp.Fetch("", nil)
}
