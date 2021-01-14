package registry

import (
	"errors"
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	"github.com/bazelbuild/bzlmod/fetch"
	urls "net/url"
)

// Registry represents a Bazel module registry.
type Registry interface {
	// URL returns the URL uniquely identifying the registry.
	URL() string
	// GetModuleBazel retrieves the MODULE.bazel file of the module with the given key. Returns an error wrapping
	// ErrNotFound if no such module exists in the registry.
	GetModuleBazel(key common.ModuleKey) ([]byte, error)
	// GetFetcher returns the Fetcher object which can be used to fetch the module with the given key. Returns an error
	// wrapping ErrNotFound if no such module exists in the registry.
	GetFetcher(key common.ModuleKey) (fetch.Fetcher, error)
}

var schemes = make(map[string]func(url *urls.URL) (Registry, error))

// New creates a new Registry object from its URL. The scheme of the URL determines the type of the registry.
func New(rawurl string) (Registry, error) {
	url, err := urls.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	fn := schemes[url.Scheme]
	if fn == nil {
		return nil, fmt.Errorf("unrecognized registry scheme %v", url.Scheme)
	}
	return fn(url)
}

var ErrNotFound = errors.New("module not found")

// GetModuleBazel gets the MODULE.bazel file contents for the module with the given key, using the list of
// registries with an optional override `regOverride` (use an empty string for no override).
// Returns the file contents, and the registry that actually has that module.
func GetModuleBazel(key common.ModuleKey, registries []string, regOverride string) ([]byte, Registry, error) {
	if regOverride != "" {
		reg, err := New(regOverride)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating override registry: %v", err)
		}
		moduleBazel, err := reg.GetModuleBazel(key)
		return moduleBazel, reg, err
	}

	for _, url := range registries {
		reg, err := New(url)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating registry from %q: %v", url, err)
		}
		moduleBazel, err := reg.GetModuleBazel(key)
		if errors.Is(err, ErrNotFound) {
			continue
		} else if err != nil {
			return nil, reg, err
		}
		return moduleBazel, reg, err
	}

	// The module couldn't be found in any of the registries.
	return nil, nil, fmt.Errorf("%w: %v in registries %q", ErrNotFound, key, registries)
}
