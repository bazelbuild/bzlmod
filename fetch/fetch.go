package fetch

// Fetcher contains all the information needed to "fetch" a repo. "Fetch" here is simply defined as making the contents
// of a repo available in a local directory through some means.
type Fetcher interface {
	// Fetch performs the fetch and returns the absolute path to the local directory where the fetched contents can be
	// accessed.
	// If vendorDir is non-empty, we're operating in vendoring mode; Fetch should make the contents available under
	// vendorDir if appropriate. Otherwise, Fetch is free to place the contents wherever.
	Fetch(vendorDir string) (string, error)

	// Fingerprint returns a fingerprint of the fetched contents. When the fingerprint changes, it's a signal that the
	// repo should be re-fetched. Note that the fingerprint need not necessarily be calculated from the actual bytes of
	// fetched contents.
	Fingerprint() string
}

// Wrapper wraps all known implementations of the Fetcher interface and acts as a multiplexer (only 1 member should be
// non-nil). It's useful in JSON marshalling/unmarshalling.
type Wrapper struct {
	Archive   *Archive   `json:",omitempty"`
	Git       *Git       `json:",omitempty"`
	LocalPath *LocalPath `json:",omitempty"`
}

func Wrap(f Fetcher) Wrapper {
	switch ft := f.(type) {
	case *Archive:
		return Wrapper{Archive: ft}
	case *Git:
		return Wrapper{Git: ft}
	case *LocalPath:
		return Wrapper{LocalPath: ft}
	}
	return Wrapper{}
}

func (w Wrapper) Unwrap() Fetcher {
	if w.Archive != nil {
		return w.Archive
	}
	if w.Git != nil {
		return w.Git
	}
	return w.LocalPath
}

func (w Wrapper) Fetch(vendorDir string) (string, error) {
	return w.Unwrap().Fetch(vendorDir)
}

func (w Wrapper) Fingerprint() string {
	return w.Unwrap().Fingerprint()
}

// LocalPath represents a locally available unpacked directory.
type LocalPath struct {
	Path string
}

func (lp *LocalPath) Fetch(vendorDir string) (string, error) {
	// Return the local path as-is, even in vendoring mode.
	return lp.Path, nil
}

func (lp *LocalPath) Fingerprint() string {
	// The local path never needs to be re-fetched.
	return ""
}
