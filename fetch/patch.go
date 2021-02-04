package fetch

type Patch struct {
	PatchFile  string
	PatchStrip int
}

func (p *Patch) Apply(dir string) error {
	// TODO: implement
	return nil
}
