package resolve

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	integrities "github.com/bazelbuild/bzlmod/common/integrity"
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

// Merges all given wsSettings objects, in ascending order of priority (later trumps earlier).
func mergeWsSettings(settings ...*wsSettings) *wsSettings {
	merged := &wsSettings{
		vendorDir:  "",
		registries: []string{"https://bcr.bazel.build/"}, // TODO: make this default a constant?
	}
	for _, next := range settings {
		if next == nil {
			continue
		}
		if next.vendorDir != "" {
			merged.vendorDir = next.vendorDir
		}
		if len(next.registries) > 0 {
			merged.registries = next.registries
		}
	}
	return merged
}

type threadState struct {
	module      *Module
	overrideSet OverrideSet
	wsSettings  *wsSettings
}

const tstateLocalKey = "module_bazel_tstate"

func initThreadState(t *starlark.Thread) *threadState {
	tstate := &threadState{overrideSet: OverrideSet{}}
	t.SetLocal(tstateLocalKey, tstate)
	return tstate
}

func getThreadState(t *starlark.Thread) *threadState {
	return t.Local(tstateLocalKey).(*threadState)
}

func extractStringSlice(list *starlark.List) ([]string, error) {
	if list == nil {
		return nil, nil
	}
	var r []string
	for i := 0; i < list.Len(); i++ {
		s, ok := starlark.AsString(list.Index(i))
		if !ok {
			return nil, fmt.Errorf("got %v, want string", list.Index(i).Type())
		}
		r = append(r, s)
	}
	return r, nil
}

func noOp(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	return starlark.None, nil
}

func moduleFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	if getThreadState(t).module != nil {
		return nil, fmt.Errorf("%v: can only be called once", b.Name())
	}
	module := NewModule()
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
	getThreadState(t).module = module
	return starlark.None, nil
}

func wsSettingsFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	if getThreadState(t).wsSettings != nil {
		return nil, fmt.Errorf("%v: can only be called once", b.Name())
	}
	wsSettings := &wsSettings{}
	var registries *starlark.List
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"vendor_dir?", &wsSettings.vendorDir,
		"registries?", &registries,
	); err != nil {
		return nil, err
	}
	var err error
	wsSettings.registries, err = extractStringSlice(registries)
	if err != nil {
		return nil, err
	}
	getThreadState(t).wsSettings = wsSettings
	return starlark.None, nil
}

func bazelDepFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	var depKey common.ModuleKey
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
		moduleName                string
		version                   string
		localPath                 string
		git                       string
		commit                    string
		url                       string
		integrity                 string
		reg                       string
		patchFilesList            *starlark.List
		allowMultipleVersionsList *starlark.List
		patchFiles                []string
		allowMultipleVersions     []string
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"module_name", &moduleName,
		"version?", &version,
		"local_path?", &localPath,
		"git?", &git,
		"commit?", &commit,
		"url?", &url,
		"integrity?", &integrity,
		"registry?", &reg,
		"patch_files?", &patchFilesList,
		"allow_multiple_versions?", &allowMultipleVersionsList,
	); err != nil {
		return nil, err
	}
	overrideSet := getThreadState(t).overrideSet
	if _, hasKey := overrideSet[moduleName]; hasKey {
		return nil, fmt.Errorf("override_dep called twice on the same module %v", moduleName)
	}
	allowMultipleVersions, err := extractStringSlice(allowMultipleVersionsList)
	if err != nil {
		return nil, err
	}
	patchFiles, err = extractStringSlice(patchFilesList)
	if err != nil {
		return nil, err
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
func runDiscovery(wsDir string, vendorDir string, registries []string) (*context, error) {
	thread := &starlark.Thread{
		Name:  "discovery of root",
		Print: func(thread *starlark.Thread, msg string) { fmt.Println(msg) },
	}
	tstate := initThreadState(thread)
	firstPassEnv := starlark.StringDict{
		"module":             moduleBuiltin,
		"workspace_settings": wsSettingsBuiltin,
		"bazel_dep":          bazelDepBuiltin,
		"override_dep":       overrideDepBuiltin,
	}

	moduleBazel, err := ioutil.ReadFile(filepath.Join(wsDir, "MODULE.bazel"))
	if err != nil {
		return nil, err
	}
	if _, err = starlark.ExecFile(thread, "/MODULE.bazel", moduleBazel, firstPassEnv); err != nil {
		return nil, err
	}

	wsSettings := mergeWsSettings(tstate.wsSettings, &wsSettings{
		vendorDir:  vendorDir,
		registries: registries,
	})
	ctx := &context{
		rootModuleName: tstate.module.Key.Name,
		depGraph: DepGraph{
			common.ModuleKey{tstate.module.Key.Name, ""}: tstate.module,
		},
		overrideSet:          tstate.overrideSet,
		moduleBazelIntegrity: integrities.MustGenerate("sha256", moduleBazel),
		vendorDir:            wsSettings.vendorDir,
	}
	if _, exists := ctx.overrideSet[ctx.rootModuleName]; exists {
		return nil, fmt.Errorf("invalid override found for root module")
	}
	ctx.overrideSet[ctx.rootModuleName] = LocalPathOverride{Path: wsDir}

	if err = processModuleDeps(tstate.module, ctx.overrideSet, ctx.depGraph, wsSettings.registries); err != nil {
		return nil, err
	}
	return ctx, nil
}

func processModuleDeps(module *Module, overrideSet OverrideSet, depGraph DepGraph, registries []string) error {
	// Rewrite the version in `depKey` when there are certain types of overrides, to make sure that we only discover 1
	// version of that dep.
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

func processSingleDep(key common.ModuleKey, overrideSet OverrideSet, depGraph DepGraph, registries []string) error {
	if _, hasKey := depGraph[key]; hasKey {
		return nil
	}

	moduleBazelResult, err := getModuleBazel(key, overrideSet, registries)
	if err != nil {
		return err
	}

	thread := &starlark.Thread{
		Name:  fmt.Sprintf("discovery[%v]", key),
		Print: func(thread *starlark.Thread, msg string) { fmt.Println(msg) },
	}
	tstate := initThreadState(thread)
	env := starlark.StringDict{
		"module":    moduleBuiltin,
		"bazel_dep": bazelDepBuiltin,

		"workspace_settings": wsSettingsNoOp,
		"override_dep":       overrideDepNoOp,
	}

	if _, err = starlark.ExecFile(thread, key.Name+"/MODULE.bazel", moduleBazelResult.moduleBazel, env); err != nil {
		return err
	}

	if tstate.module == nil {
		return fmt.Errorf("the MODULE.bazel file of %v has no module() directive", key)
	}
	if key.Name != tstate.module.Key.Name {
		return fmt.Errorf("the MODULE.bazel file of %v declares a different name (%v)", key, tstate.module.Key.Name)
	}
	if key.Version != "" && key.Version != tstate.module.Key.Version {
		return fmt.Errorf("the MODULE.bazel file of %v declares a different version (%v)", key, tstate.module.Key.Version)
	}
	tstate.module.Reg = moduleBazelResult.reg
	tstate.module.Fetcher = moduleBazelResult.fetcher
	depGraph[key] = tstate.module
	if err = processModuleDeps(tstate.module, overrideSet, depGraph, registries); err != nil {
		return err
	}
	return nil
}

type getModuleBazelResult struct {
	moduleBazel []byte
	// exactly one of fetcher and reg is nil.
	reg     registry.Registry
	fetcher fetch.Fetcher
}

// getModuleBazel grabs the MODULE.bazel file for the given key, taking into account the appropriate override and the
// list of registries. In addition to returning the MODULE.bazel file contents or an error, it also returns the origin
// registry of the module (if the module is from a registry) or the fetcher for the module (if otherwise).
func getModuleBazel(key common.ModuleKey, overrideSet OverrideSet, registries []string) (result getModuleBazelResult, err error) {
	override := overrideSet[key.Name]
	switch override.(type) {
	case LocalPathOverride, URLOverride, GitOverride:
		// For these overrides, there's no registry involved; we can concoct our own fetcher.
		switch o := override.(type) {
		case LocalPathOverride:
			result.fetcher = &fetch.LocalPath{Path: o.Path}
		case URLOverride:
			result.fetcher = &fetch.Archive{
				URLs:        []string{o.URL},
				Integrity:   o.Integrity,
				StripPrefix: "", // TODO
				PatchFiles:  o.Patches,
				Fprint:      common.Hash("urlOverride", o.URL, o.Patches),
			}
		case GitOverride:
			result.fetcher = &fetch.Git{
				Repo:       o.Repo,
				Commit:     o.Commit,
				PatchFiles: o.Patches,
			}
		}
		// Fetch the contents of the module to get to the MODULE.bazel file. Note that we specify an empty vendorDir
		// even if we're in vendoring mode: this is because this module might not end up being selected, in which case
		// we don't want the module contents cluttering up the vendor directory. Plus, we don't know what the repo name
		// of this module is!
		var path string
		path, err = result.fetcher.Fetch("")
		if err != nil {
			err = fmt.Errorf("error fetching module %q with override: %v", key.Name, err)
			return
		}
		result.moduleBazel, err = ioutil.ReadFile(filepath.Join(path, "MODULE.bazel"))
		return
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
		result.moduleBazel, result.reg, err = registry.GetModuleBazel(key, registries, regOverride)
		return
	}
}
