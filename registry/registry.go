package registry

import (
	"errors"
	"fmt"
	"github.com/bazelbuild/bzlmod/fetch"
	urlpkg "net/url"
)

type Registry interface {
	URL() string
	GetModuleBazel(name string, version string) ([]byte, error)
	GetFetcher(name string, version string) (fetch.Fetcher, error)
}

var schemes = make(map[string]func(url string) (Registry, error))

func New(url string) (Registry, error) {
	u, err := urlpkg.Parse(url)
	if err != nil {
		return nil, err
	}
	fn := schemes[u.Scheme]
	if fn == nil {
		return nil, fmt.Errorf("unrecognized registry scheme %v", u.Scheme)
	}
	return fn(url)
}

var ErrNotFound = errors.New("module not found")

// Gets the MODULE.bazel file contents for the module with the given `name` and `version`, using the list of
// registries with an optional override `regOverride` (use empty string for no override).
// Returns the file contents, and the registry that actually has that module.
func GetModuleBazel(name string, version string, registries []string, regOverride string) ([]byte, Registry, error) {
	if regOverride != "" {
		reg, err := New(regOverride)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating override registry: %v", err)
		}
		moduleBazel, err := reg.GetModuleBazel(name, version)
		return moduleBazel, reg, err
	}

	for _, url := range registries {
		reg, err := New(url)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating registry from %q: %v", url, err)
		}
		moduleBazel, err := reg.GetModuleBazel(name, version)
		if errors.Is(err, ErrNotFound) {
			continue
		} else if err != nil {
			return nil, reg, err
		}
		return moduleBazel, reg, err
	}

	// The module couldn't be found in any of the registries.
	return nil, nil, fmt.Errorf("%w: %v@%v", ErrNotFound, name, version)
}
