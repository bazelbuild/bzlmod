package resolve

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

type myReg struct {
	moduleBazel map[ModuleKey][]byte
}

func (r *myReg) addModuleBazel(t *testing.T, name string, version string, moduleBazel string) {
	if r.moduleBazel == nil {
		r.moduleBazel = make(map[ModuleKey][]byte)
	}
	if _, exists := r.moduleBazel[ModuleKey{name, version}]; exists {
		t.Fatalf("entry already exists for %v@%v", name, version)
	}
	r.moduleBazel[ModuleKey{name, version}] = []byte(moduleBazel)
}

func (r *myReg) GetModuleBazel(name string, version string, registry string) ([]byte, error) {
	moduleBazel, exists := r.moduleBazel[ModuleKey{name, version}]
	if exists {
		return moduleBazel, nil
	}
	return nil, fmt.Errorf("no such module: %v@%v", name, version)
}

func writeLocalModuleBazel(t *testing.T, dir string, moduleBazel string) {
	if err := os.MkdirAll(dir, 0777); err != nil {
		t.Fatal(err)
	}
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
	reg := &myReg{}
	reg.addModuleBazel(t, "B", "1.0", `
module(name="B", version="1.0")
bazel_dep(name="D", version="0.1")
`)
	reg.addModuleBazel(t, "C", "2.0", `
module(name="C", version="2.0")
bazel_dep(name="D", version="0.1")
`)
	reg.addModuleBazel(t, "D", "0.1", `
module(name="D", version="0.1")
`)

	v, err := Discovery(wsDir, reg)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "A", v.RootModuleName)
	assert.Equal(t, OverrideSet{"A": LocalPathOverride{Path: wsDir}}, v.OverrideSet)
	assert.Equal(t, DepGraph{
		ModuleKey{"A", ""}: &Module{
			Key: ModuleKey{"A", ""},
			Deps: map[string]ModuleKey{
				"B": {"B", "1.0"},
				"C": {"C", "2.0"},
			},
		},
		ModuleKey{"B", "1.0"}: &Module{
			Key: ModuleKey{"B", "1.0"},
			Deps: map[string]ModuleKey{
				"D": {"D", "0.1"},
			},
		},
		ModuleKey{"C", "2.0"}: &Module{
			Key: ModuleKey{"C", "2.0"},
			Deps: map[string]ModuleKey{
				"D": {"D", "0.1"},
			},
		},
		ModuleKey{"D", "0.1"}: &Module{
			Key:  ModuleKey{"D", "0.1"},
			Deps: map[string]ModuleKey{},
		},
	}, v.DepGraph)
}

func TestLocalPathOverride(t *testing.T) {
	wsDir := t.TempDir()
	wsDirA := filepath.Join(wsDir, "A")
	wsDirB := filepath.Join(wsDir, "B")
	writeLocalModuleBazel(t, wsDirA, fmt.Sprintf(`
module(name="A")
bazel_dep(name="B", version="1.0")
override_dep(module_name="B", local_path="%v")
`, wsDirB))
	writeLocalModuleBazel(t, wsDirB, `
module(name="B", version="not-sure-yet")
`)
	reg := &myReg{}
	reg.addModuleBazel(t, "B", "1.0", `
module(name="B", version="1.0")
`)

	v, err := Discovery(wsDirA, reg)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "A", v.RootModuleName)
	assert.Equal(t, OverrideSet{
		"A": LocalPathOverride{Path: wsDirA},
		"B": LocalPathOverride{Path: wsDirB},
	}, v.OverrideSet)
	assert.Equal(t, DepGraph{
		ModuleKey{"A", ""}: &Module{
			Key: ModuleKey{"A", ""},
			Deps: map[string]ModuleKey{
				"B": {"B", ""},
			},
		},
		ModuleKey{"B", ""}: &Module{
			Key:  ModuleKey{"B", "not-sure-yet"},
			Deps: map[string]ModuleKey{},
		},
	}, v.DepGraph)
}
