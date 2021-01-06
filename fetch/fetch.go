package fetch

// Fetcher contains all the information needed to "fetch" a repo. "Fetch" here is simply defined as making the contents
// of a repo available in a local directory through some means.
type Fetcher interface {
	// Performs the fetch and returns the local directory path at which the fetched contents can be accessed.
	// If vendorDir is non-empty, we're operating in vendoring mode; Fetch should make the contents available under
	// vendorDir if appropriate. Otherwise, Fetch is free to place the contents wherever.
	Fetch(vendorDir string) (string, error)
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

// LocalPath represents a locally available unpacked directory.
type LocalPath struct {
	Path string
}

func (lp *LocalPath) Fetch(vendorDir string) (string, error) {
	return lp.Path, nil
}
