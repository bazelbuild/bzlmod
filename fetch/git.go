package fetch

import "fmt"

// Git represents a Git repository.
type Git struct {
	Repo       string
	Commit     string
	PatchFiles []string
}

func (g *Git) Fetch(vendorDir string) (string, error) {
	return "", fmt.Errorf("git fetch unimplemented")
}

func (g *Git) Fingerprint() string {
	return "TODO" // TODO
}
