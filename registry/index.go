package registry

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

func (i *Index) GetFetchInfo(name string, version string) (interface{}, error) {
	panic("implement me")
}

func (i *Index) Fetch(fetchInfo interface{}, path string) error {
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
