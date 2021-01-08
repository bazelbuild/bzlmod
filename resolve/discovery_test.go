package resolve

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	integrities "github.com/bazelbuild/bzlmod/common/integrity"
	"github.com/bazelbuild/bzlmod/common/testutil"
	"github.com/bazelbuild/bzlmod/fetch"
	"github.com/bazelbuild/bzlmod/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
)

func TestDiscovery_SimpleDiamond(t *testing.T) {
	wsDir := t.TempDir()
	moduleBazel := []byte(`
module(name="A")
bazel_dep(name="B", version="1.0")
bazel_dep(name="C", version="2.0")
`)
	testutil.WriteFileBytes(t, filepath.Join(wsDir, "MODULE.bazel"), moduleBazel)
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

	v, err := runDiscovery(wsDir, "", []string{reg.URL()})
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
	v.moduleBazelIntegrity = integrities.MustGenerate("sha256", moduleBazel)
}

func TestDiscovery_RegistriesFlag(t *testing.T) {
	wsDir := t.TempDir()
	reg1 := registry.NewFake("1")
	reg2 := registry.NewFake("2")
	testutil.WriteFile(t, filepath.Join(wsDir, "MODULE.bazel"), fmt.Sprintf(`
module(name="A")
bazel_dep(name="B", version="1.0")
workspace_settings(registries=["%v"])
`, reg1.URL()))
	reg1.AddModule(t, "B", "1.0", `
module(name="B", version="1.0")
bazel_dep(name="C", version="1.0")
`, nil)
	reg1.AddModule(t, "C", "1.0", `module(name="C", version="1.0")`, nil)

	reg2.AddModule(t, "B", "1.0", `
module(name="B", version="1.0")
bazel_dep(name="C", version="2.0")
`, nil)
	reg2.AddModule(t, "C", "2.0", `module(name="C", version="2.0")`, nil)

	// If no registries are specified by flags, we use what's in workspace_settings (which is reg1).
	v, err := runDiscovery(wsDir, "", nil)
	if assert.NoError(t, err) {
		assert.Contains(t, v.depGraph, common.ModuleKey{"C", "1.0"})
		assert.NotContains(t, v.depGraph, common.ModuleKey{"C", "2.0"})
	}

	// Otherwise, the flags take precedence.
	v, err = runDiscovery(wsDir, "", []string{reg2.URL()})
	if assert.NoError(t, err) {
		assert.Contains(t, v.depGraph, common.ModuleKey{"C", "2.0"})
		assert.NotContains(t, v.depGraph, common.ModuleKey{"C", "1.0"})
	}
}

func TestDiscovery_LocalPathOverride(t *testing.T) {
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

	v, err := runDiscovery(wsDirA, "", []string{reg.URL()})
	require.NoError(t, err)
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

func TestDiscovery_URLOverride(t *testing.T) {
	fetch.TestBzlmodDir = t.TempDir()
	defer func() { fetch.TestBzlmodDir = "" }()

	reg := registry.NewFake("fake")
	reg.AddModule(t, "B", "1.0", `
module(name="B", version="1.0")
bazel_dep(name="C", version="1.0")
`, nil)
	reg.AddModule(t, "C", "1.0", `
module(name="C", version="1.0")
`, nil)
	reg.AddModule(t, "D", "1.0", `
module(name="D", version="1.0")
`, nil)

	zipArchive := testutil.BuildZipArchive(t, map[string][]byte{
		"MODULE.bazel": []byte(`
module(name="B", version="3.0")  # doesn't match what's in A's MODULE.bazel; doesn't matter
bazel_dep(name="D", version="1.0")
`),
	})
	zipIntegrity := integrities.MustGenerate("sha256", zipArchive)
	server := testutil.StaticHttpServer(map[string][]byte{
		"/b.zip": zipArchive,
	})
	defer server.Close()

	wsDir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(wsDir, "MODULE.bazel"), fmt.Sprintf(`
module(name="A")
bazel_dep(name="B", version="1.0")
override_dep(module_name="B", url="%v/b.zip", integrity="%v")
`, server.URL, zipIntegrity))

	v, err := runDiscovery(wsDir, "", []string{reg.URL()})
	require.NoError(t, err)
	assert.Equal(t, "A", v.rootModuleName)
	assert.Equal(t, OverrideSet{
		"A": LocalPathOverride{Path: wsDir},
		"B": URLOverride{
			URL:       server.URL + "/b.zip",
			Integrity: zipIntegrity,
		},
	}, v.overrideSet)
	assert.Equal(t, DepGraph{
		common.ModuleKey{"A", ""}: &Module{
			Key: common.ModuleKey{"A", ""},
			Deps: map[string]common.ModuleKey{
				"B": {"B", ""},
			},
		},
		common.ModuleKey{"B", ""}: &Module{
			Key: common.ModuleKey{"B", "3.0"},
			Deps: map[string]common.ModuleKey{
				"D": {"D", "1.0"},
			},
			Fetcher: &fetch.Archive{
				URLs:      []string{server.URL + "/b.zip"},
				Integrity: zipIntegrity,
				Fprint:    common.Hash("urlOverride", server.URL+"/b.zip", []string{}),
			},
		},
		common.ModuleKey{"D", "1.0"}: &Module{
			Key:  common.ModuleKey{"D", "1.0"},
			Deps: map[string]common.ModuleKey{},
			Reg:  reg,
		},
	}, v.depGraph)
}

// TODO: GitOverride test

func TestDiscovery_SingleVersionOverride(t *testing.T) {
	wsDir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(wsDir, "MODULE.bazel"), `
module(name="A")
bazel_dep(name="B", version="3.0")
override_dep(module_name="B", version="1.0", patch_files=["http://patches.com/patch1","http://patches.com/patch2"])
`)
	reg := registry.NewFake("fake")
	reg.AddModule(t, "B", "1.0", `
module(name="B", version="1.0")
`, nil)
	// Note that there's no B@3.0 at all. But it should be fine since it was overridden.

	v, err := runDiscovery(wsDir, "", []string{reg.URL()})
	require.NoError(t, err)
	assert.Equal(t, "A", v.rootModuleName)
	assert.Equal(t, OverrideSet{
		"A": LocalPathOverride{Path: wsDir},
		"B": SingleVersionOverride{
			Version: "1.0",
			Patches: []string{"http://patches.com/patch1", "http://patches.com/patch2"},
		},
	}, v.overrideSet)
	assert.Equal(t, DepGraph{
		common.ModuleKey{"A", ""}: &Module{
			Key: common.ModuleKey{"A", ""},
			Deps: map[string]common.ModuleKey{
				"B": {"B", "1.0"},
			},
		},
		common.ModuleKey{"B", "1.0"}: &Module{
			Key:  common.ModuleKey{"B", "1.0"},
			Deps: map[string]common.ModuleKey{},
			Reg:  reg,
		},
	}, v.depGraph)
}

func TestDiscovery_MultipleVersionsOverride(t *testing.T) {
	reg := registry.NewFake("fake")
	reg.AddModule(t, "B", "3.0", `
module(name="B", version="3.0")
`, nil)
	reg.AddModule(t, "B", "4.0", `
module(name="B", version="4.0")
`, nil)

	wsDir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(wsDir, "MODULE.bazel"), fmt.Sprintf(`
module(name="A")
bazel_dep(name="B", version="3.0", repo_name="B3")
bazel_dep(name="B", version="4.0", repo_name="B4")
override_dep(module_name="B", allow_multiple_versions=["3.3", "4.4"], registry="%v")
`, reg.URL()))

	v, err := runDiscovery(wsDir, "", nil)
	require.NoError(t, err)
	assert.Equal(t, "A", v.rootModuleName)
	assert.Equal(t, OverrideSet{
		"A": LocalPathOverride{Path: wsDir},
		"B": MultipleVersionsOverride{
			Versions: []string{"3.3", "4.4"},
			Registry: reg.URL(),
		},
	}, v.overrideSet)
	assert.Equal(t, DepGraph{
		common.ModuleKey{"A", ""}: &Module{
			Key: common.ModuleKey{"A", ""},
			Deps: map[string]common.ModuleKey{
				"B3": {"B", "3.0"},
				"B4": {"B", "4.0"},
			},
		},
		common.ModuleKey{"B", "3.0"}: &Module{
			Key:  common.ModuleKey{"B", "3.0"},
			Deps: map[string]common.ModuleKey{},
			Reg:  reg,
		},
		common.ModuleKey{"B", "4.0"}: &Module{
			Key:  common.ModuleKey{"B", "4.0"},
			Deps: map[string]common.ModuleKey{},
			Reg:  reg,
		},
	}, v.depGraph)
}

func TestDiscovery_RegistryOverride(t *testing.T) {
	// Setup: A -> {B, C}; B -> D; C -> D; D -> E.
	// A is the root module. B and E are only in reg1. C is only in reg2. D is only in reg 3.
	// reg1 is the default registry; we rely on overrides to find C and D.
	// If everything doesn't work as we expected, there will be a "module not found" error somewhere.

	reg1 := registry.NewFake("1")
	reg1.AddModule(t, "B", "1.0", `
module(name="B", version="1.0")
bazel_dep(name="D", version="1.0")
`, nil)
	reg1.AddModule(t, "E", "1.0", `
module(name="E", version="1.0")
`, nil)
	reg2 := registry.NewFake("2")
	reg2.AddModule(t, "C", "1.0", `
module(name="C", version="1.0")
bazel_dep(name="D", version="1.0")
`, nil)
	reg3 := registry.NewFake("3")
	reg3.AddModule(t, "D", "1.0", `
module(name="D", version="1.0")
bazel_dep(name="E", version="1.0")
`, nil)

	wsDir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(wsDir, "MODULE.bazel"), fmt.Sprintf(`
module(name="A")
bazel_dep(name="B", version="1.0")
bazel_dep(name="C", version="1.0")
override_dep(module_name="C", registry="%v")
override_dep(module_name="D", registry="%v")
`, reg2.URL(), reg3.URL()))

	v, err := runDiscovery(wsDir, "", []string{reg1.URL()})
	require.NoError(t, err)
	assert.Equal(t, "A", v.rootModuleName)
	assert.Equal(t, OverrideSet{
		"A": LocalPathOverride{Path: wsDir},
		"C": RegistryOverride{Registry: reg2.URL()},
		"D": RegistryOverride{Registry: reg3.URL()},
	}, v.overrideSet)
	assert.Equal(t, DepGraph{
		common.ModuleKey{"A", ""}: &Module{
			Key: common.ModuleKey{"A", ""},
			Deps: map[string]common.ModuleKey{
				"B": {"B", "1.0"},
				"C": {"C", "1.0"},
			},
		},
		common.ModuleKey{"B", "1.0"}: &Module{
			Key: common.ModuleKey{"B", "1.0"},
			Deps: map[string]common.ModuleKey{
				"D": {"D", "1.0"},
			},
			Reg: reg1,
		},
		common.ModuleKey{"C", "1.0"}: &Module{
			Key: common.ModuleKey{"C", "1.0"},
			Deps: map[string]common.ModuleKey{
				"D": {"D", "1.0"},
			},
			Reg: reg2,
		},
		common.ModuleKey{"D", "1.0"}: &Module{
			Key: common.ModuleKey{"D", "1.0"},
			Deps: map[string]common.ModuleKey{
				"E": {"E", "1.0"},
			},
			Reg: reg3,
		},
		common.ModuleKey{"E", "1.0"}: &Module{
			Key:  common.ModuleKey{"E", "1.0"},
			Deps: map[string]common.ModuleKey{},
			Reg:  reg1,
		},
	}, v.depGraph)
}
