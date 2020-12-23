package registry

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/fetch"
	urlpkg "net/url"
	"testing"
)

type nameAndVersion struct {
	name    string
	version string
}

type moduleBazelAndFetcher struct {
	moduleBazel []byte
	fetcher     fetch.Fetcher
}

type Fake struct {
	name        string
	moduleBazel map[nameAndVersion]moduleBazelAndFetcher
}

var fakes = make(map[string]*Fake)

func NewFake(name string) *Fake {
	fake := &Fake{name, make(map[nameAndVersion]moduleBazelAndFetcher)}
	fakes[name] = fake
	return fake
}

func (f *Fake) URL() string {
	return fmt.Sprintf("fake:%v", f.name)
}

func (f *Fake) AddModule(t *testing.T, name string, version string, moduleBazel string, fetcher fetch.Fetcher) {
	if _, exists := f.moduleBazel[nameAndVersion{name, version}]; exists {
		t.Fatalf("entry already exists for %v@%v", name, version)
	}
	f.moduleBazel[nameAndVersion{name, version}] = moduleBazelAndFetcher{
		moduleBazel: []byte(moduleBazel),
		fetcher:     fetcher,
	}
}

func (f *Fake) GetModuleBazel(name string, version string) ([]byte, error) {
	module, ok := f.moduleBazel[nameAndVersion{name, version}]
	if !ok {
		return nil, fmt.Errorf("%w: %v@%v", ErrNotFound, name, version)
	}
	return module.moduleBazel, nil
}

func (f *Fake) GetFetcher(name string, version string) (fetch.Fetcher, error) {
	module, ok := f.moduleBazel[nameAndVersion{name, version}]
	if !ok {
		return nil, fmt.Errorf("%w: %v@%v", ErrNotFound, name, version)
	}
	return module.fetcher, nil
}

func fakeScheme(url string) (Registry, error) {
	u, err := urlpkg.Parse(url)
	if err != nil {
		return nil, err
	}
	fake := fakes[u.Opaque]
	if fake == nil {
		return nil, fmt.Errorf("unknown fake registry: %v", u.Opaque)
	}
	return fake, nil
}

func init() {
	schemes["fake"] = fakeScheme
}
