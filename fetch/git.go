package fetch

import "fmt"

// Git represents a Git repository.
type Git struct {
	Repo    string
	Commit  string
	Patches []Patch
}

func (g *Git) Fetch(vendorDir string) (string, error) {
	return "", fmt.Errorf("git fetch unimplemented")
}

func (g *Git) Fingerprint() string {
	return "TODO" // TODO
}

func (g *Git) AppendPatches(patches []Patch) error {
	g.Patches = append(g.Patches, patches...)
	return nil
}
