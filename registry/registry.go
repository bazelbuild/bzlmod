package registry

import (
)

type RegistryHandler interface {
  GetModuleBazel(name string, version string, registry string) ([]byte, error)
}

