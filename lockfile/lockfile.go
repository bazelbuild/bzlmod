package lockfile

import "github.com/bazelbuild/bzlmod/fetch"

const FileName = "bzlmod.lock"

type Repo struct {
	Fetcher fetch.Wrapper
}

type Workspace struct {
	VendorDir string
	Repos     map[string]*Repo
}

func NewWorkspace() *Workspace {
	return &Workspace{Repos: make(map[string]*Repo)}
}
