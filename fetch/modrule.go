package fetch

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	"github.com/bazelbuild/bzlmod/common/starutil"
	"github.com/bazelbuild/bzlmod/modrule"
	"go.starlark.net/starlark"
	"log"
)

// ModRule represents a repo to be fetched by running the fetch_fn of a module rule.
type ModRule struct {
	// DefModuleKey is the key of the module that defined the module rule.
	DefModuleKey common.ModuleKey
	// DefRepoName is the repo name of the module that defined the module rule.
	DefRepoName string
	// ModuleRuleExports is the module_rule_exports parameter of the module that defined the module rule.
	ModuleRuleExports string
	// RulesetName is the name of the ruleset whose fetch_fn is to be run.
	RulesetName string
	// RepoInfo is the serialized info returned for this repo by the resolve_fn of the module rule.
	RepoInfo starutil.ValueHolder
	// Fprint is the fingerprint of this repo. Not named Fingerprint to avoid a clash with the method name.
	Fprint string
}

func (m *ModRule) Fetch(repoName string, env *Env) (string, error) {
	// TODO: Don't put into vendorDir if the module rule is machine-specific.
	eval := modrule.NewEval(env.LabelResolver)
	rulesets, err := eval.ExecForRulesets(m.DefModuleKey, m.DefRepoName, m.ModuleRuleExports)
	if err != nil {
		return "", err
	}
	ruleset, ok := rulesets[m.RulesetName]
	if !ok {
		return "", fmt.Errorf("module %v does not export a ruleset named %q", m.DefModuleKey, m.RulesetName)
	}
	thread := &starlark.Thread{
		Name: fmt.Sprintf("fetch_fn of %v in %v", ruleset.Name, ruleset.ModuleKey),
	}
	ctx := modrule.NewFetchContext(repoName, m.RepoInfo.Value)
	_, err = starlark.Call(thread, ruleset.FetchFn, []starlark.Value{ctx}, nil)
	if err != nil {
		log.Printf("%v: %v", thread.CallFrame(0).Pos, err)
		return "", fmt.Errorf("error running %v: %v", thread.Name, err)
	}
	return "something", nil // TODO
}

func (m *ModRule) Fingerprint() string {
	return m.Fprint
}

func (m *ModRule) AppendPatches(patches []Patch) error {
	return fmt.Errorf("ModRule fetcher does not support patches")
}
