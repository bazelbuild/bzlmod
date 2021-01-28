package fetch

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	"github.com/bazelbuild/bzlmod/common/starutil"
	"github.com/bazelbuild/bzlmod/modrule"
	"go.starlark.net/starlark"
	"log"
	"path/filepath"
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
	RepoInfo *starutil.ValueHolder
	// MachineSpecific records whether the module rule is machine-specific, in which case we don't place this repo in
	// the vendor dir even in vendoring mode.
	MachineSpecific bool
	// Fprint is the fingerprint of this repo. Not named Fingerprint to avoid a clash with the method name.
	Fprint string
}

func (m *ModRule) Fetch(repoName string, env *Env) (string, error) {
	// Compute where the repo should be placed.
	var repoPath string
	if env.VendorDir == "" || m.MachineSpecific {
		bzlmodWsDir, err := BzlmodWsDir(env.WsDir)
		if err != nil {
			return "", err
		}
		repoPath = filepath.Join(bzlmodWsDir, repoName)
	} else {
		repoPath = filepath.Join(env.VendorDir, repoName)
	}

	// If the dir at repoPath already exists and has the right fingerprint, our job is done here.
	if verifyFingerprintFile(repoPath, m.Fprint) {
		return repoPath, nil
	}

	// Call the fetch function.
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
	ctx := modrule.NewFetchContext(repoName, m.RepoInfo.Value, repoPath)
	_, err = starlark.Call(thread, ruleset.FetchFn, []starlark.Value{ctx}, nil)
	if err != nil {
		log.Printf("%v: %v", thread.CallFrame(0).Pos, err)
		return "", fmt.Errorf("error running %v: %v", thread.Name, err)
	}

	// Now record the fingerprint and return.
	if err = writeFingerprintFile(repoPath, m.Fprint); err != nil {
		return "", fmt.Errorf("failed to write fingerprint file: %v", err)
	}
	return repoPath, nil
}

func (m *ModRule) Fingerprint() string {
	return m.Fprint
}

func (m *ModRule) AppendPatches(_ []Patch) error {
	return fmt.Errorf("ModRule fetcher does not support patches")
}
