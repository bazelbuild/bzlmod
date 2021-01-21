package fetch

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	"github.com/bazelbuild/bzlmod/common/starutil"
)

// ModRule represents a repo to be fetched by running the fetch_fn of a module rule.
type ModRule struct {
	ModuleKey   common.ModuleKey
	RulesetName string
	RepoInfo    starutil.ValueHolder
	Fprint      string
}

func (m *ModRule) Fetch(vendorDir string) (string, error) {
	// TODO: Don't put into vendorDir if the module rule is machine-specific.
	// Wait, we might need to fetch another repo here... @^#!%^*
	panic("implement me")
}

func (m *ModRule) Fingerprint() string {
	return m.Fprint
}

func (m *ModRule) AppendPatches(patches []Patch) error {
	return fmt.Errorf("ModRule fetcher does not support patches")
}
