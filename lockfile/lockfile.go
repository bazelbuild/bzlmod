package lockfile

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	"github.com/bazelbuild/bzlmod/fetch"
	"github.com/bazelbuild/bzlmod/modrule"
	"path/filepath"
)

const FileName = "bzlmod.lock"

type Workspace struct {
	VendorDir     string
	RootRepoName  string
	Repos         map[string]*Repo
	Toolchains    []string
	ExecPlatforms []string

	// Not serialized; set by InitFetchEnv
	fetchEnv *fetch.Env
}

type Repo struct {
	Fetcher fetch.Wrapper
	Deps    map[string]string

	// Importantly, the path is *not* serialized. Normally one needs to fetch a repo to know its path. But during the
	// course of one bzlmod run, it can be wasteful to run the fetch method every time a repo's path needs to be known
	// (in particular, during module rule eval). We can use this field as a cache.
	path string
}

func NewWorkspace() *Workspace {
	// TODO: immediately init fetch env?
	return &Workspace{Repos: make(map[string]*Repo)}
}

func NewRepo() *Repo {
	return &Repo{Deps: make(map[string]string)}
}

func (ws *Workspace) InitFetchEnv(wsDir string) {
	ws.fetchEnv = &fetch.Env{
		VendorDir:     ws.VendorDir,
		WsDir:         wsDir,
		LabelResolver: ws,
	}
}

func (ws *Workspace) Fetch(repoName string) (string, error) {
	repo := ws.Repos[repoName]
	if repo == nil {
		return "", fmt.Errorf("no such repo: %v", repoName)
	}
	if repo.path != "" {
		// Short circuit if we've already fetched this repo during the course of this bzlmod run.
		return repo.path, nil
	}
	var err error
	repo.path, err = repo.Fetcher.Fetch(repoName, ws.fetchEnv)
	if err != nil {
		return "", fmt.Errorf("error fetching repo %v: %w", repoName, err)
	}
	return repo.path, nil
}

func (ws *Workspace) ResolveLabel(curRepo string, curPackage string, label *common.Label) (*modrule.ResolveLabelResult, error) {
	result := &modrule.ResolveLabelResult{}
	if !label.HasRepo {
		// Stay in the same repo.
		result.Repo = curRepo
		if !label.HasPackage {
			// Stay in the same package.
			result.Package = curPackage
		} else {
			result.Package = label.Package
		}
	} else {
		if label.Repo == "" {
			// Special case: "@//some/package" is equivalent to "@rootRepoName//some/package"
			result.Repo = ws.RootRepoName
		} else {
			var ok bool
			result.Repo, ok = ws.Repos[curRepo].Deps[label.Repo]
			if !ok {
				return nil, fmt.Errorf("no repo named %q visible from repo %q", label.Repo, curRepo)
			}
		}
		result.Package = label.Package
	}
	repoPath, err := ws.Fetch(result.Repo)
	if err != nil {
		return nil, err
	}
	result.Filename = filepath.Join(repoPath, filepath.FromSlash(result.Package), filepath.FromSlash(label.Target))
	return result, nil
}
