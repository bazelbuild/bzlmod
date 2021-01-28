package resolve

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	"github.com/hashicorp/go-version"
)

func runSelection(ctx *context) error {
	// TODO: take care of compatibility level and multiple version override

	// `selected` keeps track of the latest version of each module.
	// Note that the empty string is a "trump" version that wins over anything else (this indicates an override). It's
	// stored as `nil` in `selected` since version.Version doesn't support an empty string.
	selected := make(map[string]*version.Version)
	for key := range ctx.depGraph {
		if key.Version == "" {
			selected[key.Name] = nil
			continue
		}
		v, exists := selected[key.Name]
		if exists && v == nil {
			continue
		}
		newV, err := version.NewVersion(key.Version)
		if err != nil {
			return fmt.Errorf("can't parse version for module %v: %v", key.Name, err)
		}
		if !exists || newV.GreaterThan(v) {
			selected[key.Name] = newV
		}
	}

	// Now go over the depGraph and rewrite deps to point to the selected version. Non-selected versions are removed
	// from the graph.
	for key, module := range ctx.depGraph {
		v, exists := selected[key.Name]
		if !exists {
			return fmt.Errorf("this should never happen, but nothing is selected for module %v", key.Name)
		}
		if (v == nil && key.Version != "") || (v != nil && v.Original() != key.Version) {
			// key.Version is not selected for key.Name. Nuke!
			delete(ctx.depGraph, key)
			continue
		}
		for repoName, depKey := range module.Deps {
			v, exists := selected[depKey.Name]
			if !exists {
				return fmt.Errorf("this should never happen, but nothing is selected for module %v", depKey.Name)
			}
			if v == nil {
				module.Deps[repoName] = common.ModuleKey{depKey.Name, ""}
			} else {
				module.Deps[repoName] = common.ModuleKey{depKey.Name, v.Original()}
			}
		}
	}

	// Further remove unreferenced modules from the graph. This can still happen in cases like the following:
	// A1 -> B1, C1
	// B1 -> D1
	// C1 -> D2
	// D1 -> E1
	// D2 -> ()
	// E1 -> ()
	// Here E1 would still remain in the graph (since it's selected for E) but nobody depends on it anymore since D1
	// is no longer in the graph.
	// We can do this step by collecting all deps transitively from the root module.
	transitive := make(map[common.ModuleKey]bool)
	collectDeps(common.ModuleKey{ctx.rootModuleName, ""}, ctx.depGraph, transitive)
	for key := range ctx.depGraph {
		if !transitive[key] {
			delete(ctx.depGraph, key)
		}
	}

	return nil
}

func collectDeps(key common.ModuleKey, depGraph DepGraph, transitive map[common.ModuleKey]bool) {
	if transitive[key] {
		// Already collected.
		return
	}
	transitive[key] = true
	for _, depKey := range depGraph[key].Deps {
		collectDeps(depKey, depGraph, transitive)
	}
}
