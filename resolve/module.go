package resolve

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/fetch"
	"github.com/bazelbuild/bzlmod/registry"
)

type ModuleKey struct {
	Name    string
	Version string // empty for modules with LocalPath/URL/Git overrides
}

func (k ModuleKey) String() string {
	if k.Version == "" {
		return fmt.Sprintf("%v@_", k.Name)
	}
	return fmt.Sprintf("%v@%v", k.Name, k.Version)
}

type Module struct {
	// Fields from module()
	Key               ModuleKey
	CompatLevel       int
	BazelCompat       string
	ModuleRuleExports string
	Toolchains        []string
	ExecPlatforms     []string

	// Deps come from bazel_dep(). The key type is the repo_name
	Deps map[string]ModuleKey

	// The registry that the module comes from. Can be nil if an override exists
	Reg registry.Registry

	// These are (potentially) filled post-selection
	Fetcher  fetch.Fetcher // If an override exists, this can be filled during discovery
	RepoName string

	// Tags come from module rule invocations
	//tags []Tags
}

type DepGraph map[ModuleKey]*Module

/// Overrides

type OverrideSet map[string]interface{}

type SingleVersionOverride struct {
	Version  string
	Registry string
	Patches  []string
}

type MultipleVersionsOverride struct {
	Versions []string
	Registry string
}

type RegistryOverride struct {
	Registry string
	Patches  []string
}

type LocalPathOverride struct {
	Path string
}

type UrlOverride struct {
	Url       string
	Integrity string
	Patches   []string
}

type GitOverride struct {
	Repo    string
	Commit  string
	Patches []string
}
