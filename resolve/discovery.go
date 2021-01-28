package resolve

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	integrities "github.com/bazelbuild/bzlmod/common/integrity"
	"github.com/bazelbuild/bzlmod/common/starutil"
	"github.com/bazelbuild/bzlmod/fetch"
	"github.com/bazelbuild/bzlmod/lockfile"
	"github.com/bazelbuild/bzlmod/modrule"
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
	// Populated by the `module` directive. Is nil to begin with.
	module *Module
	// Populated by the `workspace_settings` directive. Is nil to begin with.
	wsSettings *wsSettings
	// Populated by the `bazel_dep` directive. Is empty to begin with.
	bazelDeps map[string]common.ModuleKey
	// Populated by the `override_dep` directive. Is empty to begin with.
	overrideSet OverrideSet
	// This field represents the key of the module whose MODULE.bazel is being executed on this thread. It differs from
	// the Key field of the `module` field above in that it's the "requested" key from the root module's perspective
	// (for example, the version could be empty because of an override).
	// It should be populated before the starlark.ExecFile call, *except* for the root module itself, whose name we
	// don't know until after. For the root module itself, this field should simply be kept empty (since we can only
	// fill it after starlark.ExecFile, at which point the threadState struct is no longer useful).
	intendedKey common.ModuleKey
}

const tstateLocalKey = "module_bazel_tstate"

func initThreadState(t *starlark.Thread, intendedKey common.ModuleKey) *threadState {
	tstate := &threadState{
		bazelDeps:   make(map[string]common.ModuleKey),
		overrideSet: OverrideSet{},
		intendedKey: intendedKey,
	}
	t.SetLocal(tstateLocalKey, tstate)
	return tstate
}

func getThreadState(t *starlark.Thread) *threadState {
	return t.Local(tstateLocalKey).(*threadState)
}

func extractPatchSlice(list *starlark.List, patchStrip int) ([]fetch.Patch, error) {
	if list == nil {
		if patchStrip > 0 {
			return nil, fmt.Errorf("patch_strip specified without patch_files")
		}
		return nil, nil
	}
	var patches []fetch.Patch
	for i := 0; i < list.Len(); i++ {
		s, ok := starlark.AsString(list.Index(i))
		if !ok {
			return nil, fmt.Errorf("got %v, want string", list.Index(i).Type())
		}
		patches = append(patches, fetch.Patch{s, patchStrip})
	}
	return patches, nil
}

type builtinFn func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error)

func noOp(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	return starlark.None, nil
}

func moduleFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	tstate := getThreadState(t)
	if tstate.module != nil {
		return nil, fmt.Errorf("%v: can only be called once", b.Name())
	}
	module := &Module{}
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"name?", &module.Key.Name,
		"version?", &module.Key.Version,
		"compatibility_level?", &module.CompatLevel,
		"bazel_compatibility?", &module.BazelCompat,
		"module_rule_exports?", &module.ModuleRuleExports,
		"toolchains_to_register?", &module.Toolchains,
		"execution_platforms_to_register?", &module.ExecPlatforms,
	); err != nil {
		return nil, err
	}
	tstate.module = module
	if tstate.intendedKey.Name != "" {
		// This is not the root module. tstate.intendedKey should hold the key of this module in the dep graph (it could
		// be different from module.Key in case of overrides).
		return modrule.NewBazelDep(tstate.intendedKey), nil
	} else {
		// This is the root module.
		return modrule.NewBazelDep(module.Key), nil
	}
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
	wsSettings.registries, err = starutil.ExtractStringSlice(registries)
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
		return nil, err
	}
	if repoName == "" {
		repoName = depKey.Name
	}
	getThreadState(t).bazelDeps[repoName] = depKey
	return modrule.NewBazelDep(depKey), nil // TODO: might need to rewrite the tag, since this depKey is not necessarily what's eventually used.
}

func overrideDepFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	var (
		moduleName     string
		overrideHolder *starlarkOverrideHolder
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"module_name", &moduleName,
		"override", &overrideHolder,
	); err != nil {
		return nil, err
	}
	overrideSet := getThreadState(t).overrideSet
	if _, hasKey := overrideSet[moduleName]; hasKey {
		return nil, fmt.Errorf("override_dep called twice on the same module %v", moduleName)
	}
	overrideSet[moduleName] = overrideHolder.override
	return starlark.None, nil
}

type starlarkOverrideHolder struct {
	override interface{}
}

func (s *starlarkOverrideHolder) String() string       { return fmt.Sprintf("%+v", s.override) }
func (s *starlarkOverrideHolder) Type() string         { return "override" }
func (s *starlarkOverrideHolder) Freeze()              {}
func (s *starlarkOverrideHolder) Truth() starlark.Bool { return true }
func (s *starlarkOverrideHolder) Hash() (uint32, error) {
	return 0, fmt.Errorf("not hashable: override")
}

func singleVersionOverrideFn(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	var (
		err        error
		override   SingleVersionOverride
		patchFiles *starlark.List
		patchStrip int
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"version?", &override.Version,
		"registry?", &override.Registry,
		"patch_files?", &patchFiles,
		"patch_strip?", &patchStrip,
	); err != nil {
		return nil, err
	}
	override.Patches, err = extractPatchSlice(patchFiles, patchStrip)
	if err != nil {
		return nil, err
	}
	return &starlarkOverrideHolder{override}, nil
}

func multipleVersionOverrideFn(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	var (
		err          error
		override     MultipleVersionOverride
		versionsList *starlark.List
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"versions", &versionsList,
		"registry?", &override.Registry,
	); err != nil {
		return nil, err
	}
	override.Versions, err = starutil.ExtractStringSlice(versionsList)
	if err != nil {
		return nil, err
	}
	return &starlarkOverrideHolder{override}, nil
}

func archiveOverrideFn(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	var (
		err        error
		override   ArchiveOverride
		patchFiles *starlark.List
		patchStrip int
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"url", &override.URL,
		"integrity", &override.Integrity,
		"strip_prefix?", &override.StripPrefix,
		"patch_files?", &patchFiles,
		"patch_strip?", &patchStrip,
	); err != nil {
		return nil, err
	}
	override.Patches, err = extractPatchSlice(patchFiles, patchStrip)
	if err != nil {
		return nil, err
	}
	return &starlarkOverrideHolder{override}, nil
}

func gitOverrideFn(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	var (
		err        error
		override   GitOverride
		patchFiles *starlark.List
		patchStrip int
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"repo", &override.Repo,
		"commit", &override.Commit,
		"patch_files?", &patchFiles,
		"patch_strip?", &patchStrip,
	); err != nil {
		return nil, err
	}
	override.Patches, err = extractPatchSlice(patchFiles, patchStrip)
	if err != nil {
		return nil, err
	}
	return &starlarkOverrideHolder{override}, nil
}

func localPathOverrideFn(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	var (
		err      error
		override LocalPathOverride
	)
	err = starlark.UnpackArgs(b.Name(), args, kwargs, "path", &override.Path)
	if err != nil {
		return nil, err
	}
	return &starlarkOverrideHolder{override}, nil
}

func newStarlarkEnv(isRootModule bool) starlark.StringDict {
	noOpUnlessRootModule := func(f builtinFn) builtinFn {
		if isRootModule {
			return f
		}
		return noOp
	}
	return starlark.StringDict{
		"module":                    starlark.NewBuiltin("module", moduleFn),
		"workspace_settings":        starlark.NewBuiltin("workspace_settings", noOpUnlessRootModule(wsSettingsFn)),
		"bazel_dep":                 starlark.NewBuiltin("bazel_dep", bazelDepFn),
		"override_dep":              starlark.NewBuiltin("override_dep", noOpUnlessRootModule(overrideDepFn)),
		"single_version_override":   starlark.NewBuiltin("single_version_override", noOpUnlessRootModule(singleVersionOverrideFn)),
		"multiple_version_override": starlark.NewBuiltin("multiple_version_override", noOpUnlessRootModule(multipleVersionOverrideFn)),
		"archive_override":          starlark.NewBuiltin("archive_override", noOpUnlessRootModule(archiveOverrideFn)),
		"git_override":              starlark.NewBuiltin("git_override", noOpUnlessRootModule(gitOverrideFn)),
		"local_path_override":       starlark.NewBuiltin("local_path_override", noOpUnlessRootModule(localPathOverrideFn)),
	}
}

// Run discovery. This step involves downloading and evaluating the MODULE.bazel files of all transitive
// bazel_deps.
// `wsDir` is the workspace directory, and `registries` is the list of registries to use (takes precedence
// over the registries specified in `workspace_settings`).
func runDiscovery(wsDir string, vendorDir string, registries []string) (*context, error) {
	thread := &starlark.Thread{
		Name:  "discovery of root",
		Print: func(thread *starlark.Thread, msg string) { fmt.Println(msg) },
	}
	// We can't know the key of the root module before evaluating its MODULE.bazel file!
	tstate := initThreadState(thread, common.ModuleKey{})

	moduleBazel, err := ioutil.ReadFile(filepath.Join(wsDir, "MODULE.bazel"))
	if err != nil {
		return nil, err
	}
	if _, err = starlark.ExecFile(thread, "/MODULE.bazel", moduleBazel, newStarlarkEnv(true)); err != nil {
		return nil, fmt.Errorf("%v: %v", thread.CallFrame(0).Pos, err)
	}

	if tstate.module == nil {
		tstate.module = &Module{}
	}
	tstate.module.Deps = tstate.bazelDeps
	tstate.module.Fetcher = &fetch.LocalPath{Path: ""}
	tstate.module.Tags = modrule.GetTags(thread)
	rootModuleName := tstate.module.Key.Name
	wsSettings := mergeWsSettings(tstate.wsSettings, &wsSettings{
		vendorDir:  vendorDir,
		registries: registries,
	})
	ctx := &context{
		rootModuleName: rootModuleName,
		depGraph: DepGraph{
			common.ModuleKey{rootModuleName, ""}: tstate.module,
		},
		overrideSet:          tstate.overrideSet,
		moduleBazelIntegrity: integrities.MustGenerate("sha256", moduleBazel),
		lfWorkspace:          lockfile.NewWorkspace(wsSettings.vendorDir, wsDir, rootModuleName),
	}
	if _, ok := ctx.overrideSet[rootModuleName]; ok {
		return nil, fmt.Errorf("invalid override found for root module")
	}
	ctx.overrideSet[rootModuleName] = LocalPathOverride{Path: ""}

	discovery := discovery{
		overrideSet: ctx.overrideSet,
		depGraph:    ctx.depGraph,
		registries:  wsSettings.registries,
		wsDir:       wsDir,
	}
	if err = discovery.processModuleDeps(tstate.bazelDeps); err != nil {
		return nil, err
	}
	return ctx, nil
}

type discovery struct {
	overrideSet OverrideSet
	depGraph    DepGraph
	registries  []string
	wsDir       string
}

func (d *discovery) processModuleDeps(deps map[string]common.ModuleKey) error {
	// Rewrite the version in `depKey` when there are certain types of overrides, to make sure that we only discover 1
	// version of that dep.
	for depRepoName, depKey := range deps {
		switch o := d.overrideSet[depKey.Name].(type) {
		case SingleVersionOverride:
			if o.Version != "" {
				depKey.Version = o.Version
			}
		case LocalPathOverride, ArchiveOverride, GitOverride:
			depKey.Version = ""
		}
		deps[depRepoName] = depKey
	}
	for _, depKey := range deps {
		if err := d.processSingleDep(depKey); err != nil {
			return err
		}
	}
	return nil
}

func (d *discovery) processSingleDep(key common.ModuleKey) error {
	if _, hasKey := d.depGraph[key]; hasKey {
		return nil
	}

	moduleBazelResult, err := d.getModuleBazel(key)
	if err != nil {
		return err
	}

	thread := &starlark.Thread{
		Name:  fmt.Sprintf("discovery[%v]", key),
		Print: func(thread *starlark.Thread, msg string) { fmt.Println(msg) },
	}
	tstate := initThreadState(thread, key)

	if _, err = starlark.ExecFile(thread, key.String()+"/MODULE.bazel", moduleBazelResult.moduleBazel, newStarlarkEnv(false)); err != nil {
		return fmt.Errorf("%v: %v", thread.CallFrame(0).Pos, err)
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
	tstate.module.Deps = tstate.bazelDeps
	tstate.module.Tags = modrule.GetTags(thread)
	tstate.module.Reg = moduleBazelResult.reg
	tstate.module.Fetcher = moduleBazelResult.fetcher
	d.depGraph[key] = tstate.module
	if err = d.processModuleDeps(tstate.bazelDeps); err != nil {
		return err
	}
	return nil
}

type getModuleBazelResult struct {
	moduleBazel []byte
	// exactly one of fetcher and reg is nil.
	reg     registry.Registry
	fetcher fetch.EarlyFetcher
}

// getModuleBazel grabs the MODULE.bazel file for the given key, taking into account the appropriate override and the
// list of registries. In addition to returning the MODULE.bazel file contents or an error, it also returns the origin
// registry of the module (if the module is from a registry) or the fetcher for the module (if otherwise).
func (d *discovery) getModuleBazel(key common.ModuleKey) (result getModuleBazelResult, err error) {
	override := d.overrideSet[key.Name]
	switch override.(type) {
	case LocalPathOverride, ArchiveOverride, GitOverride:
		// For these overrides, there's no registry involved; we can concoct our own fetcher.
		switch o := override.(type) {
		case LocalPathOverride:
			result.fetcher = &fetch.LocalPath{Path: o.Path}
		case ArchiveOverride:
			result.fetcher = &fetch.Archive{
				URLs:        []string{o.URL},
				Integrity:   o.Integrity,
				StripPrefix: o.StripPrefix,
				Patches:     o.Patches,
				Fprint:      common.Hash("urlOverride", o.URL, o.Patches),
			}
		case GitOverride:
			result.fetcher = &fetch.Git{
				Repo:    o.Repo,
				Commit:  o.Commit,
				Patches: o.Patches,
			}
		}
		// Fetch the contents of the module to get to the MODULE.bazel file. Note that we can only use early fetch here:
		// we don't yet know whether this module will be selected, so we don't yet have a repo name, so no vendoring,
		// no resolving labels, etc.
		var path string
		path, err = result.fetcher.EarlyFetch(d.wsDir)
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
		case MultipleVersionOverride:
			regOverride = o.Registry
		case SingleVersionOverride:
			regOverride = o.Registry
		}
		result.moduleBazel, result.reg, err = registry.GetModuleBazel(key, d.registries, regOverride)
		return
	}
}
