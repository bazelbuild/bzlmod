package resolve

import (
  "fmt"
  "io/ioutil"
  "path/filepath"

  "github.com/bazelbuild/bzlmod/registry"

  "go.starlark.net/starlark"
)

type DiscoveryResult struct {
  RootModule ModuleKey
  DepGraph
  OverrideSet
}

func noOp(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
  return starlark.None, nil
}

func Discovery(wsDir string, reg registry.RegistryHandler) (DiscoveryResult, error) {
  thread := &starlark.Thread{
    Name: "bzlmod_resolve",
    Print: func(thread *starlark.Thread, msg string) { fmt.Println(msg) },
  }

  overrideSet := OverrideSet{}
  rootModuleName := ""

  firstPassEnv := starlark.StringDict{
    "module": starlark.NewBuiltin("module", func (t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
      if len(args) > 0 {
        return nil, fmt.Errorf("Unexpected positional arguments for module()")
      }
      if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name?", &rootModuleName); err != nil {
        return nil, err
      }
      return starlark.None, nil
    }),
    "workspace_settings": starlark.NewBuiltin("workspace_settings", noOp),  // TODO
    "bazel_dep": starlark.NewBuiltin("bazel_dep", noOp),
    "override_dep": starlark.NewBuiltin("override_dep", func(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
      if len(args) > 0 {
        return nil, fmt.Errorf("Unexpected positional arguments for override_dep()")
      }
      var (
        moduleName string
        version string
        localPath string
        registry string
        git string
        commit string
        url string
        integrity []string
        patchFiles []string
        allowMultipleVersions []string
      )
      if err := starlark.UnpackArgs(b.Name(), args, kwargs, "module_name", &moduleName, "version?", &version, "local_path?", &localPath, "registry?", &registry, "git?", &git, "commit?", &commit, "url?", &url, "integrity?", &integrity, "patch_files?", &patchFiles, "allow_multiple_versions?", &allowMultipleVersions); err != nil {
        return nil, err
      }
      if _, hasKey := overrideSet[moduleName]; hasKey {
        return nil, fmt.Errorf("override_dep called twice on the same module %v", moduleName)
      }
      if version != "" {
        overrideSet[moduleName] = SingleVersionOverride{ Version: version, Registry: registry, Patches: patchFiles }
      } else if len(allowMultipleVersions) > 0 {
        overrideSet[moduleName] = MultipleVersionsOverride{ Versions: allowMultipleVersions, Registry: registry }
      } else if localPath != "" {
        overrideSet[moduleName] = LocalPathOverride { Path: localPath }
      } else if registry != "" {
        overrideSet[moduleName] = RegistryOverride { Registry: registry, Patches: patchFiles }
      } else if git != "" {
        overrideSet[moduleName] = GitOverride { Repo: git, Commit: commit, Patches: patchFiles }
      } else if url != "" {
        overrideSet[moduleName] = UrlOverride { Url: url, Integrity: integrity, Patches: patchFiles }
      } else if len(patchFiles) != 0 {
        overrideSet[moduleName] = PatchesOverride { Patches: patchFiles }
      } else {
        return nil, fmt.Errorf("nothing overridden?")
      }
      return starlark.None, nil
    }),
  }

  var data []byte
  var err error
  if data, err = ioutil.ReadFile(filepath.Join(wsDir, "MODULE.bazel")); err != nil {
    return DiscoveryResult{}, err
  }
  if _, err = starlark.ExecFile(thread, "MODULE.bazel", data, firstPassEnv); err != nil {
    return DiscoveryResult{}, err
  }

  overrideSet[rootModuleName] = LocalPathOverride { Path: wsDir }  // TODO: error if an override for the root module already exists
  rootModule := ModuleKey{ Name: rootModuleName, Version: "" }
  depGraph := DepGraph{}

  if err = process(rootModule, overrideSet, depGraph, thread, reg); err != nil {
    return DiscoveryResult{}, err
  }

  return DiscoveryResult{
    RootModule: rootModule,
    DepGraph: depGraph,
    OverrideSet: overrideSet,
  }, nil
}

func process(key ModuleKey, overrideSet OverrideSet, depGraph DepGraph, thread *starlark.Thread, reg registry.RegistryHandler) error {
  if _, hasKey := depGraph[key]; hasKey {
    return nil
  }

  curModule := &Module{ Key: key, Deps: make(map[string]ModuleKey) }
  depGraph[key] = curModule
  var moduleBazel []byte
  var err error
  if moduleBazel, err = getModuleBazel(key, overrideSet[key.Name], reg); err != nil {
    return err
  }

  env := starlark.StringDict{
    "module": starlark.NewBuiltin("module", noOp),  // TODO: fill random metadataa; check name/version match key
    "workspace_settings": starlark.NewBuiltin("workspace_settings", noOp),
    "bazel_dep": starlark.NewBuiltin("bazel_dep", func(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
      if len(args) > 0 {
        return nil, fmt.Errorf("Unexpected positional arguments for bazel_dep()")
      }
      var depKey ModuleKey
      if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name", &depKey.Name, "version", &depKey.Version); err != nil {
        return nil, err
      }
      // Rewrite the version in `depKey` when there are certain types of
      // overrides, to make sure that we only discover 1 version of that dep.
      switch o := overrideSet[depKey.Name].(type) {
      case SingleVersionOverride:
        depKey.Version = o.Version
      case LocalPathOverride, UrlOverride, GitOverride:
        depKey.Version = ""
      }
      if err := process(depKey, overrideSet, depGraph, thread, reg); err != nil {
        return nil, err
      }
      curModule.Deps[depKey.Name] = depKey
      return starlark.None, nil  // TODO: return a smart value for module rules
    }),
    "override_dep": starlark.NewBuiltin("override_dep", noOp),
  }

  if _, err = starlark.ExecFile(thread, key.Name + "/MODULE.bazel", moduleBazel, env); err != nil {
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
    return /*TODO*/reg.GetModuleBazel(key.Name, key.Version, /*TODO: ws.Registries,*/ o.Registry())
  default:
    return /*TODO*/reg.GetModuleBazel(key.Name, key.Version, /*TODO: ws.Registries,*/ "")
  }
}

