package lockfile

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/fetch"
	"path/filepath"
)

const FileName = "bzlmod.lock"

type Workspace struct {
	VendorDir string
	Repos     map[string]*Repo
}

type Repo struct {
	Fetcher fetch.Wrapper
	Deps    map[string]string
}

func NewWorkspace() *Workspace {
	return &Workspace{Repos: make(map[string]*Repo)}
}

func NewRepo() *Repo {
	return &Repo{Deps: make(map[string]string)}
}

func (ws *Workspace) Fetch(repoName string) (string, error) {
	repo := ws.Repos[repoName]
	if repo == nil {
		return "", fmt.Errorf("no such repo: %v", repoName)
	}
	if ws.VendorDir == "" {
		return repo.Fetcher.Fetch("")
	}
	return repo.Fetcher.Fetch(filepath.Join(ws.VendorDir, repoName))
}
