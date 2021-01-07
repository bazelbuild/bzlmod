package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	"github.com/bazelbuild/bzlmod/fetch"
	"io/ioutil"
	"net/http"
	urls "net/url"
	"os"
	"path"
	"path/filepath"
)

type Index struct {
	url *urls.URL
}

func NewIndex(url *urls.URL) (*Index, error) {
	return &Index{url}, nil
}

func (i *Index) URL() string {
	return i.url.String()
}

func (i *Index) grabFile(relPath string) ([]byte, error) {
	switch i.url.Scheme {
	case "file":
		p, err := ioutil.ReadFile(filepath.Join(filepath.FromSlash(i.url.Path), filepath.FromSlash(relPath)))
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return p, err
	case "http", "https":
		url := *i.url
		url.Path = path.Join(url.Path, relPath)
		resp, err := http.Get(url.String())
		if err != nil {
			return nil, fmt.Errorf("couldn't GET %v: %v", url.String(), err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("couldn't GET %v: got %v", url.String(), resp.Status)
		}
		return ioutil.ReadAll(resp.Body)
	default:
		return nil, fmt.Errorf("unrecognized scheme: %v", i.url.Scheme)
	}
}

func (i *Index) GetModuleBazel(key common.ModuleKey) ([]byte, error) {
	p, err := i.grabFile(path.Join(key.Name, key.Version, "MODULE.bazel"))
	if errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("%w: %v", ErrNotFound, key)
	}
	if err != nil {
		return nil, fmt.Errorf("error getting MODULE.bazel file for %v: %v", key, err)
	}
	return p, nil
}

func (i *Index) readAndParseJSON(relPath string, v interface{}) error {
	p, err := i.grabFile(relPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(p, v)
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

func (i *Index) GetFetcher(key common.ModuleKey) (fetch.Fetcher, error) {
	bazelRegistryJSON := bazelRegistryJSON{}
	if err := i.readAndParseJSON("bazel_registry.json", &bazelRegistryJSON); err != nil {
		return nil, fmt.Errorf("error reading bazel_registry.json of registry %v: %v", i.URL(), err)
	}
	sourceJSON := sourceJSON{}
	if err := i.readAndParseJSON(path.Join(key.Name, key.Version, "source.json"), &sourceJSON); err != nil {
		return nil, fmt.Errorf("error reading source.json file for %v from registry %v: %v", key, i.URL(), err)
	}
	sourceURL, err := urls.Parse(sourceJSON.URL)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL of %v from registry %v: %v", key, i.URL(), err)
	}
	fetcher := &fetch.Archive{
		// We use the module's name, version, and origin registry as the fingerprint. We don't use things such as
		// mirrors in the fingerprint since, for example, adding a mirror should not invalidate an existing download.
		Fingerprint: common.Hash("regModule", key.Name, key.Version, i.URL()),
	}
	for _, mirror := range bazelRegistryJSON.Mirrors {
		// TODO: support more sophisticated mirror formats?
		mirrorURL, err := urls.Parse(mirror)
		if err != nil {
			return nil, fmt.Errorf("error parsing mirror URL %v from registry %v: %v", mirror, i.URL(), err)
		}
		mirrorURL.Path = path.Join(mirrorURL.Path, sourceURL.Host, sourceURL.Path)
		mirrorURL.RawQuery = sourceURL.RawQuery
		fetcher.URLs = append(fetcher.URLs, mirrorURL.String())
	}
	fetcher.URLs = append(fetcher.URLs, sourceJSON.URL)
	fetcher.Integrity = sourceJSON.Integrity
	fetcher.StripPrefix = sourceJSON.StripPrefix
	for _, patchFileName := range sourceJSON.PatchFiles {
		patchFileURL := *i.url
		patchFileURL.Path = path.Join(patchFileURL.Path, key.Name, key.Version, "patches", patchFileName)
		fetcher.PatchFiles = append(fetcher.PatchFiles, patchFileURL.String())
	}
	// TODO: PatchStrip
	return fetcher, nil
}

func indexScheme(url *urls.URL) (Registry, error) {
	return NewIndex(url)
}

func init() {
	schemes["http"] = indexScheme
	schemes["https"] = indexScheme
	schemes["file"] = indexScheme
}
