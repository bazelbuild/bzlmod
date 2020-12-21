package registry

import (
	"fmt"
	urlpkg "net/url"
	"testing"
)

type nameAndVersion struct {
	name    string
	version string
}

type Fake struct {
	name        string
	moduleBazel map[nameAndVersion][]byte
}

var fakes = make(map[string]*Fake)

func NewFake(name string) *Fake {
	fake := &Fake{name, make(map[nameAndVersion][]byte)}
	fakes[name] = fake
	return fake
}

func (f *Fake) URL() string {
	return fmt.Sprintf("fake:%v", f.name)
}

func (r *Fake) AddModuleBazel(t *testing.T, name string, version string, moduleBazel string) {
	if _, exists := r.moduleBazel[nameAndVersion{name, version}]; exists {
		t.Fatalf("entry already exists for %v@%v", name, version)
	}
	r.moduleBazel[nameAndVersion{name, version}] = []byte(moduleBazel)
}

func (f *Fake) GetModuleBazel(name string, version string) ([]byte, error) {
	moduleBazel, ok := f.moduleBazel[nameAndVersion{name, version}]
	if !ok {
		return nil, fmt.Errorf("%w: %v@%v", ErrNotFound, name, version)
	}
	return moduleBazel, nil
}

func (f *Fake) GetFetchInfo(name string, version string) (interface{}, error) {
	panic("implement me")
}

func (f *Fake) Fetch(fetchInfo interface{}, path string) error {
	panic("implement me")
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
