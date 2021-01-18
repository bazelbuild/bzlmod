package resolve

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	"github.com/bazelbuild/bzlmod/modrule"
	"go.starlark.net/starlark"
	"io/ioutil"
	"log"
	"path/filepath"
)

func runModuleRules(ctx *context) error {
	// Group all tags by the key of the module rule and then the ruleset name.
	tagsByKeyAndRuleset := make(map[common.ModuleKey]map[string][]*modrule.Tag)
	for _, module := range ctx.depGraph {
		for idx := range module.Tags {
			tag := &module.Tags[idx]
			tagsByRuleset, ok := tagsByKeyAndRuleset[tag.ModuleKey]
			if !ok {
				tagsByRuleset = make(map[string][]*modrule.Tag)
				tagsByKeyAndRuleset[tag.ModuleKey] = tagsByRuleset
			}
			tagsByRuleset[tag.RulesetName] = append(tagsByRuleset[tag.RulesetName], tag)
		}
	}

	// Fetch all modules whose rules are invoked.
	repoPaths := make(map[string]string)
	for key, tagsByRuleset := range tagsByKeyAndRuleset {
		module := ctx.depGraph[key]
		var err error
		repoPaths[module.RepoName], err = ctx.lfWorkspace.Fetch(module.RepoName)
		if err != nil {
			return err
		}
		moduleExportsPath := filepath.Join(repoPaths[module.RepoName], filepath.FromSlash(module.ModuleRuleExports))
		moduleExports, err := ioutil.ReadFile(moduleExportsPath)
		if err != nil {
			return err
		}
		rulesets, err := modrule.GetRulesets(moduleExports) // TODO: loadFn
		if err != nil {
			return err
		}
		// Report any undefined rulesets.
		for ruleset, tags := range tagsByRuleset {
			if _, ok := rulesets[ruleset]; ok {
				continue
			}
			log.Printf("%v: module %v has no ruleset named %q\n", tags[0].Pos, tags[0].ModuleKey, tags[0].RulesetName)
			err = fmt.Errorf("undefined ruleset")
		}
		if err != nil {
			return err
		}
		// Now invoke the resolve fn of any invoked rulesets.
		for ruleset := range tagsByRuleset {
			resolveResult, err := callResolveFn(rulesets[ruleset].ResolveFn, key, ruleset, ctx.depGraph, ctx.rootModuleName)
			if err != nil {
				return err
			}
			// TODO: process resolveResult
			_ = resolveResult
		}
	}

	return nil
}

func callResolveFn(resolveFn starlark.Function, key common.ModuleKey, ruleset string, depGraph DepGraph, rootModuleName string) (resolveResult, error) {
	panic("aaaahhhh")
}

type bazelModule struct {
	name          starlark.String
	bazelDeps     *starlark.List
	ruleInstances *ruleInstances
}

func (bm *bazelModule) String() string        { return fmt.Sprintf("BazelModule[%v, ...]", string(bm.name)) }
func (bm *bazelModule) Type() string          { return "BazelModule" }
func (bm *bazelModule) Freeze()               { bm.bazelDeps.Freeze(); bm.ruleInstances.Freeze() }
func (bm *bazelModule) Truth() starlark.Bool  { return true }
func (bm *bazelModule) Hash() (uint32, error) { return 0, fmt.Errorf("not hashable: BazelModule") }

func (bm *bazelModule) Attr(name string) (starlark.Value, error) {
	switch name {
	case "name":
		return bm.name, nil
	case "bazel_deps":
		return bm.bazelDeps, nil
	case "rule_instances":
		return bm.ruleInstances, nil
	default:
		return nil, nil
	}
}

func (bm *bazelModule) AttrNames() []string {
	return []string{"name", "bazel_deps", "rule_instances"}
}

type ruleInstances struct {
	inst map[string]*starlark.List
}

func (ri *ruleInstances) String() string        { return "RuleInstances[...]" }
func (ri *ruleInstances) Type() string          { return "RuleInstances" }
func (ri *ruleInstances) Truth() starlark.Bool  { return true }
func (ri *ruleInstances) Hash() (uint32, error) { return 0, fmt.Errorf("not hashable: RuleInstances") }

func (ri *ruleInstances) Freeze() {
	for _, list := range ri.inst {
		list.Freeze()
	}
}

func (ri *ruleInstances) Attr(name string) (starlark.Value, error) {
	return ri.inst[name], nil
}

func (ri *ruleInstances) AttrNames() []string {
	keys := make([]string, len(ri.inst))
	i := 0
	for key := range ri.inst {
		keys[i] = key
		i++
	}
	return keys
}

type resolveResult struct {
	repos      map[string]starlark.Value
	toolchains []string
	platforms  []string
}

func (rr *resolveResult) String() string {
	panic("implement me")
}

func (rr *resolveResult) Type() string          { return "ResolveResult" }
func (rr *resolveResult) Freeze()               {}
func (rr *resolveResult) Truth() starlark.Bool  { return true }
func (rr *resolveResult) Hash() (uint32, error) { return 0, fmt.Errorf("not hashable: ResolveResult") }
