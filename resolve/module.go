package resolve

import (
	"github.com/bazelbuild/bzlmod/common"
	"github.com/bazelbuild/bzlmod/fetch"
	"github.com/bazelbuild/bzlmod/modrule"
	"github.com/bazelbuild/bzlmod/registry"
)

type Module struct {
	// Fields from module()
	// Key is the name and version declared by this module's module file. It should be noted that this could differ from
	// the key of this module in the dep graph, which is used to refer to this module throughout resolution.
	// Specifically, the latter could have an empty version when there's a non-registry override.
	Key               common.ModuleKey
	CompatLevel       int
	BazelCompat       string
	ModuleRuleExports string
	Toolchains        []string
	ExecPlatforms     []string

	// Deps come from bazel_dep(). The key type is the repo_name
	Deps map[string]common.ModuleKey

	// The registry that the module comes from. Can be nil if a non-registry override exists
	Reg registry.Registry

	// Tags come from module rule invocations.
	Tags []modrule.Tag

	// The following are (potentially) filled post-selection
	Fetcher  fetch.Fetcher // If a non-registry override exists, this is filled during discovery
	RepoName string
}

func NewModule() *Module {
	return &Module{Deps: make(map[string]common.ModuleKey)}
}

type DepGraph map[common.ModuleKey]*Module

/// Overrides

type OverrideSet map[string]interface{}

type SingleVersionOverride struct {
	Version  string
	Registry string
	Patches  []fetch.Patch
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
	Patches     []fetch.Patch
}

type GitOverride struct {
	Repo    string
	Commit  string
	Patches []fetch.Patch
}
