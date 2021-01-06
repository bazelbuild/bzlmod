package resolve

import (
	"github.com/bazelbuild/bzlmod/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSelection_SimpleDiamond(t *testing.T) {
	depGraph := DepGraph{
		common.ModuleKey{"A", ""}: &Module{
			Key: common.ModuleKey{"A", "1.0"},
			Deps: map[string]common.ModuleKey{
				"myB": {"B", ""},
				"myC": {"C", "1.0"},
			},
		},
		common.ModuleKey{"B", ""}: &Module{
			Key: common.ModuleKey{"B", "local-version"},
			Deps: map[string]common.ModuleKey{
				"myD": {"D", "1.0"},
			},
		},
		common.ModuleKey{"C", "1.0"}: &Module{
			Key: common.ModuleKey{"C", "1.0"},
			Deps: map[string]common.ModuleKey{
				"yourD": {"D", "1.1"},
			},
		},
		common.ModuleKey{"D", "1.0"}: &Module{
			Key: common.ModuleKey{"D", "1.0"},
			Deps: map[string]common.ModuleKey{
				"E": {"E", "1.0"},
			},
		},
		common.ModuleKey{"D", "1.1"}: &Module{
			Key:  common.ModuleKey{"D", "1.1"},
			Deps: map[string]common.ModuleKey{},
		},
		common.ModuleKey{"E", "1.0"}: &Module{
			Key:  common.ModuleKey{"E", "1.0"},
			Deps: map[string]common.ModuleKey{},
		},
	}
	ctx := &context{
		rootModuleName: "A",
		depGraph:       depGraph,
		overrideSet:    OverrideSet{},
	}
	require.NoError(t, Selection(ctx))
	expectedDepGraph := DepGraph{
		common.ModuleKey{"A", ""}: &Module{
			Key: common.ModuleKey{"A", "1.0"},
			Deps: map[string]common.ModuleKey{
				"myB": {"B", ""},
				"myC": {"C", "1.0"},
			},
		},
		common.ModuleKey{"B", ""}: &Module{
			Key: common.ModuleKey{"B", "local-version"},
			Deps: map[string]common.ModuleKey{
				"myD": {"D", "1.1"},
			},
		},
		common.ModuleKey{"C", "1.0"}: &Module{
			Key: common.ModuleKey{"C", "1.0"},
			Deps: map[string]common.ModuleKey{
				"yourD": {"D", "1.1"},
			},
		},
		common.ModuleKey{"D", "1.1"}: &Module{
			Key:  common.ModuleKey{"D", "1.1"},
			Deps: map[string]common.ModuleKey{},
		},
	}
	assert.Equal(t, expectedDepGraph, depGraph)
}
