package resolve

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	"github.com/bazelbuild/bzlmod/common/testutil"
	"github.com/bazelbuild/bzlmod/fetch"
	"github.com/bazelbuild/bzlmod/registry"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func TestSimpleDiamond(t *testing.T) {
	wsDir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(wsDir, "MODULE.bazel"), `
module(name="A")
bazel_dep(name="B", version="1.0")
bazel_dep(name="C", version="2.0")
`)
	reg := registry.NewFake("fake")
	reg.AddModule(t, "B", "1.0", `
module(name="B", version="1.0")
bazel_dep(name="D", version="0.1")
`, nil)
	reg.AddModule(t, "C", "2.0", `
module(name="C", version="2.0")
bazel_dep(name="D", version="0.1")
`, nil)
	reg.AddModule(t, "D", "0.1", `
module(name="D", version="0.1")
`, nil)

	v, err := Discovery(wsDir, []string{reg.URL()})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "A", v.rootModuleName)
	assert.Equal(t, OverrideSet{"A": LocalPathOverride{Path: wsDir}}, v.overrideSet)
	assert.Equal(t, DepGraph{
		common.ModuleKey{"A", ""}: &Module{
			Key: common.ModuleKey{"A", ""},
			Deps: map[string]common.ModuleKey{
				"B": {"B", "1.0"},
				"C": {"C", "2.0"},
			},
		},
		common.ModuleKey{"B", "1.0"}: &Module{
			Key: common.ModuleKey{"B", "1.0"},
			Deps: map[string]common.ModuleKey{
				"D": {"D", "0.1"},
			},
			Reg: reg,
		},
		common.ModuleKey{"C", "2.0"}: &Module{
			Key: common.ModuleKey{"C", "2.0"},
			Deps: map[string]common.ModuleKey{
				"D": {"D", "0.1"},
			},
			Reg: reg,
		},
		common.ModuleKey{"D", "0.1"}: &Module{
			Key:  common.ModuleKey{"D", "0.1"},
			Deps: map[string]common.ModuleKey{},
			Reg:  reg,
		},
	}, v.depGraph)
}

func TestLocalPathOverride(t *testing.T) {
	wsDir := t.TempDir()
	wsDirA := filepath.Join(wsDir, "A")
	wsDirB := filepath.Join(wsDir, "B")
	testutil.WriteFile(t, filepath.Join(wsDirA, "MODULE.bazel"), fmt.Sprintf(`
module(name="A")
bazel_dep(name="B", version="1.0")
override_dep(module_name="B", local_path="%v")
`, wsDirB))
	testutil.WriteFile(t, filepath.Join(wsDirB, "MODULE.bazel"), `
module(name="B", version="not-sure-yet")
`)
	reg := registry.NewFake("fake")
	reg.AddModule(t, "B", "1.0", `
module(name="B", version="1.0")
`, nil)

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
		common.ModuleKey{"A", ""}: &Module{
			Key: common.ModuleKey{"A", ""},
			Deps: map[string]common.ModuleKey{
				"B": {"B", ""},
			},
		},
		common.ModuleKey{"B", ""}: &Module{
			Key:     common.ModuleKey{"B", "not-sure-yet"},
			Deps:    map[string]common.ModuleKey{},
			Fetcher: &fetch.LocalPath{Path: wsDirB},
		},
	}, v.depGraph)
}
