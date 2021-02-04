package lockfile

import (
	"github.com/bazelbuild/bzlmod/fetch"
	"github.com/bazelbuild/bzlmod/modrule"
)

// FetcherWrapper wraps all known implementations of the fetch.Fetcher interface and acts as a multiplexer (only 1
// member should be non-nil). It's useful in JSON marshalling/unmarshalling.
type FetcherWrapper struct {
	Archive   *fetch.Archive   `json:",omitempty"`
	Git       *fetch.Git       `json:",omitempty"`
	LocalPath *fetch.LocalPath `json:",omitempty"`
	ModRule   *modrule.Fetcher `json:",omitempty"`
}

func WrapFetcher(f fetch.Fetcher) FetcherWrapper {
	switch ft := f.(type) {
	case *fetch.Archive:
		return FetcherWrapper{Archive: ft}
	case *fetch.Git:
		return FetcherWrapper{Git: ft}
	case *fetch.LocalPath:
		return FetcherWrapper{LocalPath: ft}
	case *modrule.Fetcher:
		return FetcherWrapper{ModRule: ft}
	}
	return FetcherWrapper{}
}

func (w FetcherWrapper) Unwrap() fetch.Fetcher {
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

func (w FetcherWrapper) Fetch(repoName string, env *fetch.Env) (string, error) {
	return w.Unwrap().Fetch(repoName, env)
}

func (w FetcherWrapper) Fingerprint() string {
	return w.Unwrap().Fingerprint()
}

func (w FetcherWrapper) AppendPatches(patches []fetch.Patch) error {
	return w.Unwrap().AppendPatches(patches)
}
