package fetch

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
)

type Context struct {
	WsDir      string
	VendorDir  string
	BzlmodRoot string
}

type Fetcher interface {
	// Performs the fetch and returns the local file path at which the fetched contents can be accessed.
	Fetch() (string, error)
	// Fetches only the MODULE.bazel file, without patches?
	// TODO: clarify what this does, and how it's different from registries
	FetchModuleBazel() ([]byte, error)
}

// Wrapper wraps all known implementations of the Fetcher interface and acts as a multiplexer (only 1 member should be
// non-nil). It's useful in JSON marshalling/unmarshalling.
type Wrapper struct {
	Http      *Http      `json:",omitempty"`
	Git       *Git       `json:",omitempty"`
	LocalPath *LocalPath `json:",omitempty"`
}

func Wrap(f Fetcher) Wrapper {
	switch ft := f.(type) {
	case *Http:
		return Wrapper{Http: ft}
	case *Git:
		return Wrapper{Git: ft}
	case *LocalPath:
		return Wrapper{LocalPath: ft}
	}
	return Wrapper{}
}

func (w Wrapper) Unwrap() Fetcher {
	if w.Http != nil {
		return w.Http
	}
	if w.Git != nil {
		return w.Git
	}
	return w.LocalPath
}

func (w Wrapper) Fetch() (string, error) {
	return w.Unwrap().Fetch()
}

func (w Wrapper) FetchModuleBazel() ([]byte, error) {
	return w.Unwrap().FetchModuleBazel()
}

type Http struct {
	Urls        []string
	Integrity   string
	StripPrefix string
	PatchFiles  []string
}

func (h *Http) Fetch() (string, error) {
	return "", fmt.Errorf("http fetch unimplemented")
}

func (h *Http) FetchModuleBazel() ([]byte, error) {
	return nil, fmt.Errorf("http fetch unimplemented")
}

type Git struct {
	Repo       string
	Commit     string
	PatchFiles []string
}

func (g *Git) Fetch() (string, error) {
	return "", fmt.Errorf("git fetch unimplemented")
}

func (g *Git) FetchModuleBazel() ([]byte, error) {
	return nil, fmt.Errorf("git fetch unimplemented")
}

type LocalPath struct {
	Path string
}

func (lp *LocalPath) Fetch() (string, error) {
	return lp.Path, nil
}

func (lp *LocalPath) FetchModuleBazel() ([]byte, error) {
	return ioutil.ReadFile(filepath.Join(lp.Path, "MODULE.bazel"))
}
