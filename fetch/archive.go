package fetch

import (
	"archive/zip"
	"fmt"
	integrities "github.com/bazelbuild/bzlmod/common/integrity"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	urls "net/url"
	"os"
	"path/filepath"
	"strings"
)

// Archive represents an archive to be fetched from one of multiple equivalent URLs.
type Archive struct {
	URLs        []string
	Integrity   string
	StripPrefix string
	PatchFiles  []string

	// Fingerprint should be a hash computed from information that is enough to distinguish this archive fetch from
	// others. It will be used as the name of the shared repo directory.
	Fingerprint string
}

func (a *Archive) Fetch(vendorDir string) (string, error) {
	// First check if the corresponding shared repo directory already exists. If it does, we can skip the downloading.
	sharedRepoDir, err := SharedRepoDir(a.Fingerprint)
	if err != nil {
		return "", err
	}
	sharedRepoDirExists := true
	_, err = os.Stat(sharedRepoDir)
	if err != nil {
		if os.IsNotExist(err) {
			sharedRepoDirExists = false
		} else {
			return "", fmt.Errorf("can't shared repo directory: %v", err)
		}
	}

	// If we're not in vendoring mode, just download into the shared repo dir if it doesn't exist, and return that
	// directory.
	if vendorDir == "" {
		if !sharedRepoDirExists {
			if err := a.downloadExtractAndPatch(sharedRepoDir); err != nil {
				return "", err
			}
		}
		return sharedRepoDir, nil
	}

	// If we're in vendoring mode, also check if the vendorDir exists and whether the fingerprint matches what we
	// expect.
	fprintFilePath := filepath.Join(vendorDir, "bzlmod.fingerprint")
	fprint, err := ioutil.ReadFile(fprintFilePath)
	if err == nil && string(fprint) == a.Fingerprint {
		// The vendor directory is properly set up.
		return vendorDir, nil
	}
	if sharedRepoDirExists {
		// Copy the entire directory over.
		if err := copyDir(sharedRepoDir, vendorDir); err != nil {
			return "", fmt.Errorf("error copying shared repo dir to vendor dir: %v", err)
		}
	} else {
		if err := a.downloadExtractAndPatch(vendorDir); err != nil {
			return "", err
		}
	}
	// Write the fingerprint file.
	if err := ioutil.WriteFile(fprintFilePath, []byte(a.Fingerprint), 0766); err != nil {
		return "", fmt.Errorf("can't write fingerprint file: %v", err)
	}
	return vendorDir, nil
}

func (a *Archive) downloadExtractAndPatch(destDir string) error {
	integ, err := integrities.NewChecker(a.Integrity)
	if err != nil {
		return err
	}

	archivePath := ""
	rawurl := ""
	for _, rawurl = range a.URLs {
		url, err := urls.Parse(rawurl)
		if err != nil {
			log.Printf("failed to parse URL: %v\n", err)
			continue
		}
		switch url.Scheme {
		case "http", "https":
			archivePath, err = cachedDownload(rawurl, integ)
		case "file":
			archivePath = filepath.FromSlash(url.Path)
			err = verifyIntegrity(archivePath, integ)
		default:
			log.Printf("unrecognized scheme: %v\n", url.Scheme)
			continue
		}
		if err == nil {
			break
		}
		log.Printf("error fetching from %v: %v\n", rawurl, err)
	}

	if archivePath == "" {
		// All our attempts to fetch from those URLs failed.
		return fmt.Errorf("error downloading archive")
	}

	// Now perform the extraction.
	// TODO: support other archive formats
	if err := extractZipFile(archivePath, destDir, a.StripPrefix); err != nil {
		return fmt.Errorf("error extracting archive downloaded from %v: %v", rawurl, err)
	}
	// TODO: patch
	return nil
}

// Downloads the given URL into the central cache location and returns the file path.
func cachedDownload(url string, integ integrities.Checker) (string, error) {
	fp, err := HTTPCacheFilePath(url)
	if err != nil {
		return "", err
	}
	// TODO: deal with concurrent access. What if another `bzlmod fetch` is executed at the same time?
	if verifyIntegrity(fp, integ) == nil {
		// This file exists in the cache and matches the specified integrity. Return its path immediately.
		return fp, nil
	}
	// The file doesn't exist in the cache, or doesn't match the given integrity. Re-download it.
	if err := os.MkdirAll(filepath.Dir(fp), 0777); err != nil {
		return "", fmt.Errorf("can't create directories for http cache: %v", err)
	}
	f, err := os.Create(fp)
	if err != nil {
		return "", fmt.Errorf("can't create http cache file: %v", err)
	}
	defer f.Close()
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	integ.Reset()
	if _, err := io.Copy(io.MultiWriter(f, integ), resp.Body); err != nil {
		return "", err
	}
	if !integ.Check() {
		return "", fmt.Errorf("failed integrity check")
	}
	return fp, nil
}

// Verifies the integrity of the file at path `fp` against the given integrity checker.
func verifyIntegrity(fp string, integ integrities.Checker) error {
	f, err := os.Open(fp)
	if err != nil {
		return err
	}
	integ.Reset()
	_, err = io.Copy(integ, f)
	if err != nil {
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}
	if !integ.Check() {
		return fmt.Errorf("failed integrity check")
	}
	return nil
}

func extractZipFile(archivePath string, destDir string, stripPrefix string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		fr, err := f.Open()
		if err != nil {
			return fmt.Errorf("can't open file for reading %v: %v", f.Name, err)
		}
		defer fr.Close()
		relPath := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(f.Name, stripPrefix)))
		absPath := filepath.Join(destDir, relPath)
		if err := os.MkdirAll(filepath.Dir(absPath), 0777); err != nil {
			return fmt.Errorf("can't create directories for %v: %v", absPath, err)
		}
		w, err := os.Create(absPath)
		if err != nil {
			return fmt.Errorf("can't create file for writing %v: %v", absPath, err)
		}
		defer w.Close()
		_, err = io.Copy(w, fr)
		if err != nil {
			return fmt.Errorf("error during I/O: %v", err)
		}
	}
	return nil
}

func copyDir(from string, to string) error {
	if err := os.RemoveAll(to); err != nil {
		return err
	}
	return filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relpath, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		r, err := os.Open(path)
		if err != nil {
			return err
		}
		defer r.Close()
		topath := filepath.Join(to, relpath)
		err = os.MkdirAll(filepath.Dir(topath), 0777)
		if err != nil {
			return err
		}
		w, err := os.Create(topath)
		if err != nil {
			return err
		}
		defer w.Close()
		_, err = io.Copy(w, r)
		if err != nil {
			return err
		}
		return nil
	})
}
