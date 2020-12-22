package resolve

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/registry"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

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
	reg := registry.NewFake("fake")
	reg.AddModuleBazel(t, "B", "1.0", `
module(name="B", version="1.0")
bazel_dep(name="D", version="0.1")
`)
	reg.AddModuleBazel(t, "C", "2.0", `
module(name="C", version="2.0")
bazel_dep(name="D", version="0.1")
`)
	reg.AddModuleBazel(t, "D", "0.1", `
module(name="D", version="0.1")
`)

	v, err := Discovery(wsDir, []string{reg.URL()})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "A", v.rootModuleName)
	assert.Equal(t, OverrideSet{"A": LocalPathOverride{Path: wsDir}}, v.overrideSet)
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
	}, v.depGraph)
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
	reg := registry.NewFake("fake")
	reg.AddModuleBazel(t, "B", "1.0", `
module(name="B", version="1.0")
`)

	v, err := Discovery(wsDirA, []string{reg.URL()})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "A", v.rootModuleName)
	assert.Equal(t, OverrideSet{
		"A": LocalPathOverride{Path: wsDirA},
		"B": LocalPathOverride{Path: wsDirB},
	}, v.overrideSet)
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
	}, v.depGraph)
}
