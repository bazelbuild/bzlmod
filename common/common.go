package common

import "fmt"

type ModuleKey struct {
	Name    string
	Version string // empty for modules with LocalPath/URL/Git overrides
}

func (k *ModuleKey) String() string {
	if k.Version == "" {
		return fmt.Sprintf("%v@_", k.Name)
	}
	return fmt.Sprintf("%v@%v", k.Name, k.Version)
}
