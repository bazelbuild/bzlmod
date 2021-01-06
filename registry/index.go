package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	"github.com/bazelbuild/bzlmod/fetch"
	"io/ioutil"
	urls "net/url"
	"os"
	"path"
	"path/filepath"
)

type FileIndex struct {
	localPath string
}

type HTTPIndex struct {
	url *urls.URL
}

func NewFileIndex(url *urls.URL) (*FileIndex, error) {
	if url.Scheme != "file" {
		return nil, fmt.Errorf("unknown scheme: %v", url.Scheme)
	}
	return &FileIndex{filepath.FromSlash(url.Path)}, nil
}

func NewHTTPIndex(url *urls.URL) (*HTTPIndex, error) {
	switch url.Scheme {
	case "http", "https":
		return nil, fmt.Errorf("http scheme not implemented yet")
	default:
		return nil, fmt.Errorf("unknown scheme: %v", url.Scheme)
	}
}

func (fi *FileIndex) url() *urls.URL {
	return &urls.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(fi.localPath),
	}
}

func (fi *FileIndex) URL() string {
	return fi.url().String()
}

func (hi *HTTPIndex) URL() string {
	return hi.url.String()
}

func (fi *FileIndex) GetModuleBazel(name string, version string) ([]byte, error) {
	bytes, err := ioutil.ReadFile(filepath.Join(fi.localPath, name, version, "MODULE.bazel"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %v@%v", ErrNotFound, name, version)
	}
	if err != nil {
		return nil, fmt.Errorf("error getting MODULE.bazel file for %v@%v: %v", name, version, err)
	}
	return bytes, nil
}

func (hi *HTTPIndex) GetModuleBazel(name string, version string) ([]byte, error) {
	u := *hi.url
	u.Path = path.Join(u.Path, name, version, "MODULE.bazel")
	panic("implement me")
}

func readAndParseJSON(filename string, v interface{}) error {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, v)
}

type bazelRegistryJSON struct {
	Mirrors []string `json:"mirrors"`
}

type sourceJSON struct {
	URL         string   `json:"url"`
	Integrity   string   `json:"integrity"`
	StripPrefix string   `json:"strip_prefix"`
	PatchFiles  []string `json:"patch_files"`
	PatchStrip  int      `json:"patch_strip"`
}

func (fi *FileIndex) GetFetcher(name string, version string) (fetch.Fetcher, error) {
	bazelRegistryJSON := bazelRegistryJSON{}
	if err := readAndParseJSON(filepath.Join(fi.localPath, "bazel_registry.json"), &bazelRegistryJSON); err != nil {
		return nil, fmt.Errorf("error reading bazel_registry.json of local registry at %v: %v", fi.localPath, err)
	}
	sourceJSON := sourceJSON{}
	if err := readAndParseJSON(filepath.Join(fi.localPath, name, version, "source.json"), &sourceJSON); err != nil {
		return nil, fmt.Errorf("error reading source.json file for %v@%v from local registry at %v: %v", name, version, fi.localPath, err)
	}
	sourceURL, err := urls.Parse(sourceJSON.URL)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL of %v@%v from local registry %v: %v", name, version, fi.localPath, err)
	}
	fetcher := &fetch.Archive{
		// We use the module's name, version, and origin registry as the fingerprint. We don't use things such as
		// mirrors in the fingerprint since, for example, adding a mirror should not invalidate an existing download.
		Fingerprint: common.Hash("regModule", name, version, fi.URL()),
	}
	for _, mirror := range bazelRegistryJSON.Mirrors {
		// TODO: support more sophisticated mirror formats?
		mirrorURL, err := urls.Parse(mirror)
		if err != nil {
			return nil, fmt.Errorf("error parsing mirror URL %v from local registry %v: %v", mirror, fi.localPath, err)
		}
		mirrorURL.Path = path.Join(mirrorURL.Path, sourceURL.Host, sourceURL.Path)
		mirrorURL.RawQuery = sourceURL.RawQuery
		fetcher.URLs = append(fetcher.URLs, mirrorURL.String())
	}
	fetcher.URLs = append(fetcher.URLs, sourceJSON.URL)
	fetcher.Integrity = sourceJSON.Integrity
	fetcher.StripPrefix = sourceJSON.StripPrefix
	for _, patchFileName := range sourceJSON.PatchFiles {
		patchFileURL := *fi.url()
		patchFileURL.Path = path.Join(patchFileURL.Path, name, version, "patches", patchFileName)
		fetcher.PatchFiles = append(fetcher.PatchFiles, patchFileURL.String())
	}
	// TODO: PatchStrip
	return fetcher, nil
}

func (hi *HTTPIndex) GetFetcher(name string, version string) (fetch.Fetcher, error) {
	u := *hi.url
	u.Path = path.Join(u.Path, name, version, "source.json")
	panic("implement me")
}

func fileIndexScheme(url *urls.URL) (Registry, error) {
	return NewFileIndex(url)
}

func httpIndexScheme(url *urls.URL) (Registry, error) {
	return NewHTTPIndex(url)
}

func init() {
	schemes["http"] = httpIndexScheme
	schemes["https"] = httpIndexScheme
	schemes["file"] = fileIndexScheme
}
