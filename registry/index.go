package registry

import "github.com/bazelbuild/bzlmod/fetch"

type Index struct {
	url string
}

func NewIndex(url string) *Index {
	return &Index{url}
}

func (i *Index) URL() string {
	return i.url
}

func (i *Index) GetModuleBazel(name string, version string) ([]byte, error) {
	panic("implement me")
}

func (i *Index) GetFetcher(name string, version string) (fetch.Fetcher, error) {
	panic("implement me")
}

func indexScheme(url string) (Registry, error) {
	return NewIndex(url), nil
}

func init() {
	schemes["http"] = indexScheme
	schemes["https"] = indexScheme
	schemes["file"] = indexScheme
}
