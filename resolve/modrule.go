package resolve

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	"github.com/bazelbuild/bzlmod/common/starutil"
	"github.com/bazelbuild/bzlmod/fetch"
	"github.com/bazelbuild/bzlmod/lockfile"
	"github.com/bazelbuild/bzlmod/modrule"
	"go.starlark.net/starlark"
	"log"
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
	eval := modrule.NewEval(ctx.lfWorkspace)
	for key, tagsByRuleset := range tagsByKeyAndRuleset {
		module := ctx.depGraph[key]
		rulesets, err := eval.ExecForRulesets(key, module.RepoName, module.ModuleRuleExports)
		if err != nil {
			return err
		}
		// Report any undefined rulesets.
		for rulesetName, tags := range tagsByRuleset {
			if _, ok := rulesets[rulesetName]; ok {
				continue
			}
			log.Printf("%v: module %v does not export a ruleset named %q\n", tags[0].Pos, key, rulesetName)
			err = fmt.Errorf("undefined ruleset")
		}
		if err != nil {
			return err
		}
		// Now invoke the resolve fn of any invoked rulesets.
		for rulesetName := range tagsByRuleset {
			resolveResult, err := callResolveFn(rulesets[rulesetName], ctx.depGraph, ctx.rootModuleName)
			if err != nil {
				return err
			}
			for repoName, repoInfo := range resolveResult.Repos {
				repo := lockfile.NewRepo()
				// Each repo created by module rules defined by module X can use all X's bazel_deps.
				// TODO: clarify and think through.
				for depRepoName, actualRepoName := range ctx.lfWorkspace.Repos[module.RepoName].Deps {
					repo.Deps[depRepoName] = actualRepoName
				}
				for depRepoName, _ := range resolveResult.Repos {
					if depRepoName != repoName {
						repo.Deps[depRepoName] = depRepoName
					}
				}
				repo.Fetcher = fetch.Wrap(&fetch.ModRule{
					DefModuleKey:      key,
					DefRepoName:       module.RepoName,
					ModuleRuleExports: module.ModuleRuleExports,
					RulesetName:       rulesetName,
					RepoInfo:          starutil.ValueHolder{repoInfo},
					Fprint:            "", // TODO
				})
				ctx.lfWorkspace.Repos[repoName] = repo
			}
			ctx.lfWorkspace.Toolchains = append(ctx.lfWorkspace.Toolchains, resolveResult.Toolchains...)
			ctx.lfWorkspace.ExecPlatforms = append(ctx.lfWorkspace.ExecPlatforms, resolveResult.ExecPlatforms...)
		}
	}

	return nil
}

func callResolveFn(ruleset *modrule.Ruleset, depGraph DepGraph, rootModuleName string) (*modrule.ResolveResult, error) {
	topModule, err := buildTopModule(ruleset, depGraph, rootModuleName)
	if err != nil {
		return nil, err
	}
	thread := &starlark.Thread{
		Name: fmt.Sprintf("resolve_fn of %v in %v", ruleset.Name, ruleset.ModuleKey),
	}
	ctx := modrule.NewResolveContext(topModule)
	result, err := starlark.Call(thread, ruleset.ResolveFn, []starlark.Value{ctx}, nil)
	if err != nil {
		log.Printf("%v: %v", thread.CallFrame(0).Pos, err)
		return nil, fmt.Errorf("error running %v: %v", thread.Name, err)
	}
	rr, ok := result.(*modrule.ResolveResult)
	if !ok {
		log.Printf("%v: expected return value of type ResolveResult, got: %v", ruleset.ResolveFn.Position(), result)
		return nil, fmt.Errorf("resolve_fn did not return a ResolveResult object")
	}
	return rr, nil
}

func buildTopModule(ruleset *modrule.Ruleset, depGraph DepGraph, rootModuleName string) (*modrule.BazelModule, error) {
	bazelModuleMap := make(map[common.ModuleKey]*modrule.BazelModule)
	for key, module := range depGraph {
		bazelModule := &modrule.BazelModule{
			Name:          starlark.String(module.Key.Name),
			Version:       starlark.String(module.Key.Version),
			RuleInstances: modrule.NewRuleInstances(),
		}
		bazelModuleMap[key] = bazelModule
		for _, tag := range module.Tags {
			// Filter tags down to those that belong to this ruleset.
			if tag.ModuleKey != ruleset.ModuleKey || tag.RulesetName != ruleset.Name {
				continue
			}
			rule := ruleset.Members[tag.RuleName]
			if rule == nil {
				log.Printf("%v: ruleset %v in module %v has no member rule named %q\n", tag.Pos, tag.RulesetName, tag.ModuleKey, tag.RuleName)
				return nil, fmt.Errorf("undefined rule")
			}
			ruleInstance, err := rule.NewInstance(tag.Kwargs)
			if err != nil {
				log.Printf("%v: %v", tag.Pos, err)
				return nil, fmt.Errorf("error creating rule instance")
			}
			bazelModule.RuleInstances.Append(tag.RuleName, ruleInstance)
		}
	}
	for key, module := range depGraph {
		bazelModule := bazelModuleMap[key]
		for _, depKey := range module.Deps {
			bazelModule.BazelDeps = append(bazelModule.BazelDeps, bazelModuleMap[depKey])
		}
	}
	return bazelModuleMap[common.ModuleKey{rootModuleName, ""}], nil
}
