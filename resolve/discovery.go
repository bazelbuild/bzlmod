package resolve

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	"github.com/bazelbuild/bzlmod/fetch"
	"io/ioutil"
	"path/filepath"

	"github.com/bazelbuild/bzlmod/registry"

	"go.starlark.net/starlark"
)

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
		git                   string
		commit                string
		url                   string
		integrity             string
		reg                   string
		patchFiles            []string
		allowMultipleVersions []string
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"module_name", &moduleName,
		"version?", &version,
		"local_path?", &localPath,
		"git?", &git,
		"commit?", &commit,
		"url?", &url,
		"integrity?", &integrity,
		"reg?", &reg,
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
	} else if git != "" {
		overrideSet[moduleName] = GitOverride{Repo: git, Commit: commit, Patches: patchFiles}
	} else if url != "" {
		overrideSet[moduleName] = URLOverride{URL: url, Integrity: integrity, Patches: patchFiles}
	} else if reg != "" || len(patchFiles) > 0 {
		overrideSet[moduleName] = RegistryOverride{Registry: reg, Patches: patchFiles}
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

// Run discovery. This step involves downloading and evaluating the MODULE.bazel files of all transitive
// bazel_deps.
// `wsDir` is the workspace directory, and `registries` is the list of registries to use (takes precedence
// over the registries specified in `workspace_settings`).
func Discovery(wsDir string, registries []string) (*context, error) {
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
	moduleBazel, err := ioutil.ReadFile(moduleBazelPath)
	if err != nil {
		return nil, err
	}
	if _, err = starlark.ExecFile(thread, moduleBazelPath, moduleBazel, firstPassEnv); err != nil {
		return nil, err
	}

	ctx := &context{
		rootModuleName: module.Key.Name,
		depGraph: DepGraph{
			ModuleKey{module.Key.Name, ""}: module,
		},
		overrideSet: getThreadState(thread).overrideSet,
	}
	if _, exists := ctx.overrideSet[ctx.rootModuleName]; exists {
		return nil, fmt.Errorf("invalid override found for root module")
	}
	ctx.overrideSet[ctx.rootModuleName] = LocalPathOverride{Path: wsDir}

	if err = processModuleDeps(module, ctx.overrideSet, ctx.depGraph, registries); err != nil {
		return nil, err
	}
	return ctx, nil
}

func processModuleDeps(module *Module, overrideSet OverrideSet, depGraph DepGraph, registries []string) error {
	// Rewrite the version in `depKey` when there are certain types of
	// overrides, to make sure that we only discover 1 version of that dep.
	for depRepoName, depKey := range module.Deps {
		switch o := overrideSet[depKey.Name].(type) {
		case SingleVersionOverride:
			depKey.Version = o.Version
		case LocalPathOverride, URLOverride, GitOverride:
			depKey.Version = ""
		}
		module.Deps[depRepoName] = depKey
	}
	for _, depKey := range module.Deps {
		if err := processSingleDep(depKey, overrideSet, depGraph, registries); err != nil {
			return err
		}
	}
	return nil
}

func processSingleDep(key ModuleKey, overrideSet OverrideSet, depGraph DepGraph, registries []string) error {
	if _, hasKey := depGraph[key]; hasKey {
		return nil
	}

	curModule := &Module{Deps: make(map[string]ModuleKey)}
	depGraph[key] = curModule
	moduleBazel, err := getModuleBazel(key, curModule, overrideSet, registries)
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
		return fmt.Errorf("the MODULE.bazel file of %v declares a different version (%v)", key, curModule.Key.Version)
	}
	if err = processModuleDeps(curModule, overrideSet, depGraph, registries); err != nil {
		return err
	}
	return nil
}

// getModuleBazel grabs the MODULE.bazel file for the given key, taking into account the appropriate override and the
// list of registries. In addition to returning the MODULE.bazel file contents or an error, it also writes the
// appropriate fetcher or registry into the provided `module` variable.
func getModuleBazel(key ModuleKey, module *Module, overrideSet OverrideSet, registries []string) (moduleBazel []byte, err error) {
	override := overrideSet[key.Name]
	switch override.(type) {
	case LocalPathOverride, URLOverride, GitOverride:
		// For these overrides, there's no registry involved; we can concoct our own fetcher.
		switch o := override.(type) {
		case LocalPathOverride:
			module.Fetcher = &fetch.LocalPath{Path: o.Path}
		case URLOverride:
			module.Fetcher = &fetch.Archive{
				URLs:        []string{o.URL},
				Integrity:   o.Integrity,
				StripPrefix: "", // TODO
				PatchFiles:  o.Patches,
				Fingerprint: common.Hash("urlOverride", o.URL, o.Patches),
			}
		case GitOverride:
			module.Fetcher = &fetch.Git{
				Repo:       o.Repo,
				Commit:     o.Commit,
				PatchFiles: o.Patches,
			}
		}
		// Fetch the contents of the module to get to the MODULE.bazel file. Note that we specify an empty vendorDir
		// even if we're in vendoring mode: this is because this module might not end up being selected, in which case
		// we don't want the module contents cluttering up the vendor directory. Plus, we don't know what the repo name
		// of this module is!
		path, err := module.Fetcher.Fetch("")
		if err != nil {
			return nil, fmt.Errorf("error fetching module %q with override: %v", key.Name, err)
		}
		return ioutil.ReadFile(filepath.Join(path, "MODULE.bazel"))
	default:
		// Otherwise, we can directly grab the MODULE.bazel file from the registry.
		regOverride := ""
		switch o := override.(type) {
		case MultipleVersionsOverride:
			regOverride = o.Registry
		case SingleVersionOverride:
			regOverride = o.Registry
		case RegistryOverride:
			regOverride = o.Registry
		}
		moduleBazel, module.Reg, err = registry.GetModuleBazel(key.Name, key.Version, registries, regOverride)
		return
	}
}
