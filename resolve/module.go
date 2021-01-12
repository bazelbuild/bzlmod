package resolve

import (
	"github.com/bazelbuild/bzlmod/common"
	"github.com/bazelbuild/bzlmod/fetch"
	"github.com/bazelbuild/bzlmod/registry"
)

type Module struct {
	// Fields from module()
	Key               common.ModuleKey
	CompatLevel       int
	BazelCompat       string
	ModuleRuleExports string
	Toolchains        []string
	ExecPlatforms     []string

	// Deps come from bazel_dep(). The key type is the repo_name
	Deps map[string]common.ModuleKey

	// The registry that the module comes from. Can be nil if an override exists
	Reg registry.Registry

	// These are (potentially) filled post-selection
	Fetcher  fetch.Fetcher // If an override exists, this can be filled during discovery
	RepoName string

	// Tags come from module rule invocations
	//tags []Tags
}

func NewModule() *Module {
	return &Module{Deps: make(map[string]common.ModuleKey)}
}

type DepGraph map[common.ModuleKey]*Module

/// Overrides

type OverrideSet map[string]interface{}

type SingleVersionOverride struct {
	Version    string
	Registry   string
	Patches    []string
	PatchStrip int
}

type MultipleVersionOverride struct {
	Versions []string
	Registry string
}

type LocalPathOverride struct {
	Path string
}

type ArchiveOverride struct {
	URL         string
	Integrity   string
	StripPrefix string
	Patches     []string
	PatchStrip  int
}

type GitOverride struct {
	Repo       string
	Commit     string
	Patches    []string
	PatchStrip int
}
