package fetch

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	integrities "github.com/bazelbuild/bzlmod/common/integrity"
	"hash"
	"io"
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
	Patches     []Patch

	// Fprint should be a hash computed from information that is enough to distinguish this archive fetch from
	// others. It will be used as the name of the shared repo directory.
	// The field is not called Fingerprint to avoid conflict with the method name.
	Fprint string
}

func (a *Archive) Fingerprint() string {
	return a.Fprint
}

func (a *Archive) AppendPatches(patches []Patch) error {
	a.Patches = append(a.Patches, patches...)
	return nil
}

func (a *Archive) EarlyFetch(_ string) (string, error) {
	return a.Fetch("", nil)
}

func (a *Archive) Fetch(repoName string, env *Env) (string, error) {
	vendorRepoDir := ""
	if env != nil && env.VendorDir != "" {
		vendorRepoDir = filepath.Join(env.VendorDir, repoName)
	}
	// If we're in vendoring mode and the vendorRepoDir exists and has the right fingerprint, return immediately.
	if vendorRepoDir != "" && VerifyFingerprintFile(vendorRepoDir, a.Fprint) {
		return vendorRepoDir, nil
	}

	// Otherwise, check if the corresponding shared repo directory exists and has the right fingerprint (in which case
	// we can skip the download).
	// It might seem redundant to check for the fingerprint as the name of the directory is itself the fingerprint;
	// however, the fingerprint file is only written if the download, extraction or patching didn't fail halfway.
	sharedRepoDir, err := SharedRepoDir(a.Fprint)
	if err != nil {
		return "", err
	}
	sharedRepoDirReady := VerifyFingerprintFile(sharedRepoDir, a.Fprint)

	// If we're not in vendoring mode, just prep the shared repo dir if it's not ready, and return that directory.
	if vendorRepoDir == "" {
		if !sharedRepoDirReady {
			if err := a.downloadExtractAndPatch(sharedRepoDir); err != nil {
				return "", err
			}
			if err := WriteFingerprintFile(sharedRepoDir, a.Fprint); err != nil {
				return "", fmt.Errorf("can't write fingerprint file: %v", err)
			}
		}
		return sharedRepoDir, nil
	}

	// If we're in vendoring mode, we should either copy from the shared repo dir if it's ready, or otherwise download
	// straight into the vendor dir.
	if sharedRepoDirReady {
		// Copy the entire directory over. Note that the fingerprint file itself is explicitly not copied, so that we
		// only write it in the end if the whole copy succeeded.
		if err := copyDirWithoutFingerprintFile(sharedRepoDir, vendorRepoDir); err != nil {
			return "", fmt.Errorf("error copying shared repo dir to vendor dir: %v", err)
		}
	} else {
		if err := a.downloadExtractAndPatch(vendorRepoDir); err != nil {
			return "", err
		}
	}
	// Write the fingerprint file.
	if err := WriteFingerprintFile(vendorRepoDir, a.Fprint); err != nil {
		return "", fmt.Errorf("can't write fingerprint file: %v", err)
	}
	return vendorRepoDir, nil
}

func (a *Archive) downloadExtractAndPatch(destDir string) error {
	result, err := Download(a.URLs, a.Integrity)
	if err != nil {
		return err
	}
	if err := Extract(result.Filename, destDir, a.StripPrefix); err != nil {
		return fmt.Errorf("error extracting archive downloaded from %v: %v", result.OrigURL, err)
	}
	for _, patch := range a.Patches {
		if err := patch.Apply(destDir); err != nil {
			return fmt.Errorf("error applying patch %v: %v", patch, err)
		}
	}
	return nil
}

type DownloadResult struct {
	// Filename is the file path to the downloaded file (most likely residing in the bzlmod HTTP cache).
	Filename string
	// OrigURL is the URL from the original list that was actually successfully downloaded.
	OrigURL string
	// Sha256 is the SHA-256 checksum of the downloaded file.
	Sha256 []byte
}

// Download tries to download the given urlList into the bzlmod HTTP cache, and returns the path to the downloaded file
// (see DownloadResult). The first URL in the list to be successfully downloaded is used. The downloaded file must match
// the given integrity. The URLs in the list can use the "file://" scheme, in which case Download doesn't perform an
// HTTP get but just uses the file on local disk.
func Download(urlList []string, integrity string) (result DownloadResult, err error) {
	integ, err := integrities.NewChecker(integrity)
	if err != nil {
		return
	}

	sideHash := sha256.New()
	for _, result.OrigURL = range urlList {
		url, err := urls.Parse(result.OrigURL)
		if err != nil {
			log.Printf("failed to parse URL: %v\n", err)
			continue
		}
		switch url.Scheme {
		case "http", "https":
			result.Filename, err = cachedHTTPDownload(result.OrigURL, integ, sideHash)
		case "file":
			result.Filename = filepath.FromSlash(url.Path)
			err = verifyIntegrity(result.Filename, integ, sideHash)
		default:
			log.Printf("unrecognized scheme: %v\n", url.Scheme)
			continue
		}
		if err == nil {
			result.Sha256 = sideHash.Sum(nil)
			break
		}
		log.Printf("error fetching from %v: %v\n", result.OrigURL, err)
	}

	if result.OrigURL == "" {
		// All our attempts to fetch from those URLs failed.
		err = fmt.Errorf("error downloading archive")
		return
	}
	return
}

// CachedHTTPDownlaod downloads the given URL into bzlmod's HTTP cache and returns the file path. If a file
// corresponding to the URL already exists in the cache and passes the integrity check, no actual download is performed.
// If the downloaded file does not pass the integrity check, an error is returned.
func cachedHTTPDownload(url string, integ integrities.Checker, sideHash hash.Hash) (string, error) {
	fp, err := HTTPCacheFilePath(url)
	if err != nil {
		return "", err
	}
	// TODO: deal with concurrent access. What if another `bzlmod fetch` is executed at the same time?
	if verifyIntegrity(fp, integ, sideHash) == nil {
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
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("got status: %v", resp.Status)
	}
	integ.Reset()
	sideHash.Reset()
	if _, err := io.Copy(io.MultiWriter(f, integ, sideHash), resp.Body); err != nil {
		return "", err
	}
	if !integ.Check() {
		return "", fmt.Errorf("failed integrity check")
	}
	return fp, nil
}

// Verifies the integrity of the file at path `fp` against the given integrity checker.
func verifyIntegrity(fp string, integ integrities.Checker, sideHash hash.Hash) error {
	f, err := os.Open(fp)
	if err != nil {
		return err
	}
	integ.Reset()
	sideHash.Reset()
	_, err = io.Copy(io.MultiWriter(integ, sideHash), f)
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

// Extract extracts the given archive file (archivePath) to destDir. It chooses the decompression algorithm based on the
// extension of the file name.
func Extract(archivePath string, destDir string, stripPrefix string) error {
	// TODO: support other archive formats
	ext := filepath.Ext(archivePath)
	switch ext {
	case ".zip":
		return extractZipFile(archivePath, destDir, stripPrefix)
	default:
		return fmt.Errorf("unsupported archive extension: %v", ext)
	}
}

func extractZipFile(archivePath string, destDir string, stripPrefix string) error {
	if err := os.RemoveAll(destDir); err != nil {
		return err
	}
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
		if err := os.MkdirAll(filepath.Dir(absPath), 0775); err != nil {
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

func copyDirWithoutFingerprintFile(from string, to string) error {
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
		if relpath == "bzlmod.fingerprint" {
			// skip the fingerprint file itself.
			return nil
		}
		r, err := os.Open(path)
		if err != nil {
			return err
		}
		defer r.Close()
		topath := filepath.Join(to, relpath)
		err = os.MkdirAll(filepath.Dir(topath), 0775)
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
