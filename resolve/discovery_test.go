package resolve

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
)

type myReg struct {
	moduleBazel map[ModuleKey][]byte
}

func (r *myReg) addModuleBazel(name string, version string, moduleBazel string) error {
	if r.moduleBazel == nil {
		r.moduleBazel = make(map[ModuleKey][]byte)
	}
	if _, exists := r.moduleBazel[ModuleKey{name, version}]; exists {
		return fmt.Errorf("entry already exists for %v@%v", name, version)
	}
	r.moduleBazel[ModuleKey{name, version}] = []byte(moduleBazel)
	return nil
}

func (r *myReg) GetModuleBazel(name string, version string, registry string) ([]byte, error) {
	moduleBazel, exists := r.moduleBazel[ModuleKey{name, version}]
	if exists {
		return moduleBazel, nil
	}
	return nil, fmt.Errorf("no such module: %v@%v", name, version)
}

func writeLocalModuleBazel(t *testing.T, dir string, moduleBazel string) {
	if err := ioutil.WriteFile(filepath.Join(dir, "MODULE.bazel"), []byte(moduleBazel), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestSimpleDiamond(t *testing.T) {
	wsDir := t.TempDir()
	writeLocalModuleBazel(t, wsDir, `
module(name="A")
bazel_dep(name="B", version="1.0")
bazel_dep(name="C", version="2.0")
`)
	var err error
	reg := &myReg{}
	err = reg.addModuleBazel("B", "1.0", `
module(name="B", version="1.0")
bazel_dep(name="D", version="0.1")
`)
	if err != nil {
		t.Fatal(err)
	}
	err = reg.addModuleBazel("C", "2.0", `
module(name="C", version="2.0")
bazel_dep(name="D", version="0.1")
`)
	if err != nil {
		t.Fatal(err)
	}
	err = reg.addModuleBazel("D", "0.1", `
module(name="D", version="0.1")
`)
	if err != nil {
		t.Fatal(err)
	}

	var v DiscoveryResult
	if v, err = Discovery(wsDir, reg); err != nil {
		t.Fatal(err)
	}
	t.Logf("%v", v)
	t.Log("all good")
}
