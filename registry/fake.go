package registry

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	"github.com/bazelbuild/bzlmod/fetch"
	urls "net/url"
	"testing"
)

type moduleBazelAndFetcher struct {
	moduleBazel []byte
	fetcher     fetch.Fetcher
}

type Fake struct {
	name        string
	moduleBazel map[common.ModuleKey]moduleBazelAndFetcher
}

var fakes = make(map[string]*Fake)

func NewFake(name string) *Fake {
	fake := &Fake{name, make(map[common.ModuleKey]moduleBazelAndFetcher)}
	fakes[name] = fake
	return fake
}

func (f *Fake) URL() string {
	return fmt.Sprintf("fake:%v", f.name)
}

func (f *Fake) AddModule(t *testing.T, name string, version string, moduleBazel string, fetcher fetch.Fetcher) {
	key := common.ModuleKey{name, version}
	if _, exists := f.moduleBazel[key]; exists {
		t.Fatalf("entry already exists for %v", key)
	}
	f.moduleBazel[key] = moduleBazelAndFetcher{
		moduleBazel: []byte(moduleBazel),
		fetcher:     fetcher,
	}
}

func (f *Fake) GetModuleBazel(key common.ModuleKey) ([]byte, error) {
	module, ok := f.moduleBazel[key]
	if !ok {
		return nil, fmt.Errorf("%w: %v", ErrNotFound, key)
	}
	return module.moduleBazel, nil
}

func (f *Fake) GetFetcher(key common.ModuleKey) (fetch.Fetcher, error) {
	module, ok := f.moduleBazel[key]
	if !ok {
		return nil, fmt.Errorf("%w: %v", ErrNotFound, key)
	}
	return module.fetcher, nil
}

func fakeScheme(url *urls.URL) (Registry, error) {
	fake := fakes[url.Opaque]
	if fake == nil {
		return nil, fmt.Errorf("unknown fake registry: %v", url.Opaque)
	}
	return fake, nil
}

func init() {
	schemes["fake"] = fakeScheme
}
