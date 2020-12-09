package resolve

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/bazelbuild/bzlmod/registry"

	"go.starlark.net/starlark"
)

type DiscoveryResult struct {
	RootModuleName string
	DepGraph
	OverrideSet
}

type wsSettings struct {
	vendorDir  string
	registries []string
}

type threadState struct {
	module      *Module
	overrideSet OverrideSet
	wsSettings  *wsSettings
}

const localKey = "modulebzl"

func getThreadState(t *starlark.Thread) *threadState {
	return t.Local(localKey).(*threadState)
}

func noOp(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	return starlark.None, nil
}

func moduleFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	module := getThreadState(t).module
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"name?", &module.Key.Name,
		"version?", &module.Key.Version,
		"compatibility_level?", &module.CompatLevel,
		"bazel_compatibility?", &module.BazelCompat,
		"module_rule_exports?", &module.ModuleRuleExports,
		"toolchains_to_register", &module.Toolchains,
		"execution_platforms_to_register", &module.ExecPlatforms,
	); err != nil {
		return nil, err
	}
	return starlark.None, nil
}

func wsSettingsFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	wsSettings := &wsSettings{}
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"vendor_dir?", &wsSettings.vendorDir,
		"registries?", &wsSettings.registries,
	); err != nil {
		return nil, err
	}
	getThreadState(t).wsSettings = wsSettings
	return starlark.None, nil
}

func bazelDepFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	var depKey ModuleKey
	var repoName string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"name", &depKey.Name,
		"version", &depKey.Version,
		"repo_name?", &repoName,
	); err != nil {
		// TODO: figure out how to include the file/line info here, same elsewhere
		return nil, err
	}
	if repoName == "" {
		repoName = depKey.Name
	}
	getThreadState(t).module.Deps[repoName] = depKey
	return starlark.None, nil // TODO: return a smart value for module rules
}

func overrideDepFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	var (
		moduleName            string
		version               string
		localPath             string
		reg                   string
		git                   string
		commit                string
		url                   string
		integrity             []string
		patchFiles            []string
		allowMultipleVersions []string
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"module_name", &moduleName,
		"version?", &version,
		"local_path?", &localPath,
		"reg?", &reg,
		"git?", &git,
		"commit?", &commit,
		"url?", &url,
		"Integrity?", &integrity,
		"patch_files?", &patchFiles,
		"allow_multiple_versions?", &allowMultipleVersions,
	); err != nil {
		return nil, err
	}
	overrideSet := getThreadState(t).overrideSet
	if _, hasKey := overrideSet[moduleName]; hasKey {
		return nil, fmt.Errorf("override_dep called twice on the same module %v", moduleName)
	}
	if version != "" {
		overrideSet[moduleName] = SingleVersionOverride{Version: version, Registry: reg, Patches: patchFiles}
	} else if len(allowMultipleVersions) > 0 {
		overrideSet[moduleName] = MultipleVersionsOverride{Versions: allowMultipleVersions, Registry: reg}
	} else if localPath != "" {
		overrideSet[moduleName] = LocalPathOverride{Path: localPath}
	} else if reg != "" {
		overrideSet[moduleName] = RegistryOverride{Registry: reg, Patches: patchFiles}
	} else if git != "" {
		overrideSet[moduleName] = GitOverride{Repo: git, Commit: commit, Patches: patchFiles}
	} else if url != "" {
		overrideSet[moduleName] = UrlOverride{Url: url, Integrity: integrity, Patches: patchFiles}
	} else if len(patchFiles) != 0 {
		overrideSet[moduleName] = PatchesOverride{Patches: patchFiles}
	} else {
		return nil, fmt.Errorf("nothing was overridden for module %v", moduleName)
	}
	return starlark.None, nil
}

var (
	moduleBuiltin      = starlark.NewBuiltin("module", moduleFn)
	wsSettingsBuiltin  = starlark.NewBuiltin("workspace_settings", wsSettingsFn)
	bazelDepBuiltin    = starlark.NewBuiltin("bazel_dep", bazelDepFn)
	overrideDepBuiltin = starlark.NewBuiltin("override_dep", overrideDepFn)

	wsSettingsNoOp  = starlark.NewBuiltin("workspace_settings", noOp)
	overrideDepNoOp = starlark.NewBuiltin("override_dep", noOp)
)

func Discovery(wsDir string, reg registry.RegistryHandler) (result DiscoveryResult, err error) {
	thread := &starlark.Thread{
		Name:  "discovery of root",
		Print: func(thread *starlark.Thread, msg string) { fmt.Println(msg) },
	}
	module := &Module{Deps: make(map[string]ModuleKey)}
	thread.SetLocal(localKey, &threadState{
		module:      module,
		overrideSet: OverrideSet{},
	})
	firstPassEnv := starlark.StringDict{
		"module":             moduleBuiltin,
		"workspace_settings": wsSettingsBuiltin,
		"bazel_dep":          bazelDepBuiltin,
		"override_dep":       overrideDepBuiltin,
	}

	moduleBazelPath := filepath.Join(wsDir, "MODULE.bazel")
	var data []byte
	if data, err = ioutil.ReadFile(moduleBazelPath); err != nil {
		return
	}
	if _, err = starlark.ExecFile(thread, moduleBazelPath, data, firstPassEnv); err != nil {
		return
	}

	result.RootModuleName = module.Key.Name
	result.OverrideSet = getThreadState(thread).overrideSet
	if _, exists := result.OverrideSet[result.RootModuleName]; exists {
		err = fmt.Errorf("invalid override found for root module")
		return
	}
	result.OverrideSet[result.RootModuleName] = LocalPathOverride{Path: wsDir}
	result.DepGraph = DepGraph{
		ModuleKey{result.RootModuleName, ""}: module,
	}

	if err = processModuleDeps(module, result.OverrideSet, result.DepGraph, reg); err != nil {
		return
	}
	return
}

func processModuleDeps(module *Module, overrideSet OverrideSet, depGraph DepGraph, reg registry.RegistryHandler) error {
	// Rewrite the version in `depKey` when there are certain types of
	// overrides, to make sure that we only discover 1 version of that dep.
	for depRepoName, depKey := range module.Deps {
		switch o := overrideSet[depKey.Name].(type) {
		case SingleVersionOverride:
			depKey.Version = o.Version
		case LocalPathOverride, UrlOverride, GitOverride:
			depKey.Version = ""
		}
		module.Deps[depRepoName] = depKey
	}
	for _, depKey := range module.Deps {
		if err := processSingleDep(depKey, overrideSet, depGraph, reg); err != nil {
			return err
		}
	}
	return nil
}

func processSingleDep(key ModuleKey, overrideSet OverrideSet, depGraph DepGraph, reg registry.RegistryHandler) error {
	if _, hasKey := depGraph[key]; hasKey {
		return nil
	}

	curModule := &Module{Deps: make(map[string]ModuleKey)}
	depGraph[key] = curModule
	moduleBazel, err := getModuleBazel(key, overrideSet[key.Name], reg)
	if err != nil {
		return err
	}

	thread := &starlark.Thread{
		Name:  fmt.Sprintf("discovery[%v]", key),
		Print: func(thread *starlark.Thread, msg string) { fmt.Println(msg) },
	}
	thread.SetLocal(localKey, &threadState{module: curModule, overrideSet: overrideSet})
	env := starlark.StringDict{
		"module":    moduleBuiltin,
		"bazel_dep": bazelDepBuiltin,

		"workspace_settings": wsSettingsNoOp,
		"override_dep":       overrideDepNoOp,
	}

	if _, err = starlark.ExecFile(thread, key.Name+"/MODULE.bazel", moduleBazel, env); err != nil {
		return err
	}

	if key.Name != curModule.Key.Name {
		return fmt.Errorf("the MODULE.bazel file of %v declares a different name (%v)", key.Name, curModule.Key.Name)
	}
	if key.Version != "" && key.Version != curModule.Key.Version {
		return fmt.Errorf("the MODULE.bazel file of %v@%v declares a different version (%v)", key.Name, key.Version, curModule.Key.Version)
	}
	if err = processModuleDeps(curModule, overrideSet, depGraph, reg); err != nil {
		return err
	}
	return nil
}

func getModuleBazel(key ModuleKey, override interface{}, reg registry.RegistryHandler) ([]byte, error) {
	switch o := override.(type) {
	case LocalPathOverride:
		return ioutil.ReadFile(filepath.Join(o.Path, "MODULE.bazel"))
	case UrlOverride:
		// TODO: download url & apply patch
		return nil, fmt.Errorf("UrlOverride unimplemented")
	case GitOverride:
		// TODO: download git & apply patch
		return nil, fmt.Errorf("GitOverride unimplemented")
	case interface{ Registry() string }:
		return /*TODO*/ reg.GetModuleBazel(key.Name, key.Version /*TODO: ws.Registries,*/, o.Registry())
	default:
		return /*TODO*/ reg.GetModuleBazel(key.Name, key.Version /*TODO: ws.Registries,*/, "")
	}
}
