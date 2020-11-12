package resolve

import (
)

type ModuleKey struct {
  Name string
  Version string  // empty for modules with LocalPath/Url/Git overrides
}

type FetchInfo interface {
  FetchInfo()
}

type Module struct {
  Key ModuleKey
  Deps map[string]ModuleKey  // the key type is the repo_name
  //tags []Tags
  FetchInfo FetchInfo
}

type DepGraph map[ModuleKey]*Module

/// Overrides

type OverrideSet map[string]interface {}

type SingleVersionOverride struct {
  Version string
  Registry string
  Patches []string
}

type MultipleVersionsOverride struct {
  Versions []string
  Registry string
}

type RegistryOverride struct {
  Registry string
  Patches []string
}

type LocalPathOverride struct {
  Path string
}

type UrlOverride struct {
  Url string
  Integrity []string
  Patches []string
}

type GitOverride struct {
  Repo string
  Commit string
  Patches []string
}

type PatchesOverride struct {
  Patches []string
}

