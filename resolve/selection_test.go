package resolve

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSelection_SimpleDiamond(t *testing.T) {
	depGraph := DepGraph{
		ModuleKey{"A", ""}: &Module{
			Key: ModuleKey{"A", "1.0"},
			Deps: map[string]ModuleKey{
				"myB": {"B", ""},
				"myC": {"C", "1.0"},
			},
		},
		ModuleKey{"B", ""}: &Module{
			Key: ModuleKey{"B", "local-version"},
			Deps: map[string]ModuleKey{
				"myD": {"D", "1.0"},
			},
		},
		ModuleKey{"C", "1.0"}: &Module{
			Key: ModuleKey{"C", "1.0"},
			Deps: map[string]ModuleKey{
				"yourD": {"D", "1.1"},
			},
		},
		ModuleKey{"D", "1.0"}: &Module{
			Key: ModuleKey{"D", "1.0"},
			Deps: map[string]ModuleKey{
				"E": {"E", "1.0"},
			},
		},
		ModuleKey{"D", "1.1"}: &Module{
			Key:  ModuleKey{"D", "1.1"},
			Deps: map[string]ModuleKey{},
		},
		ModuleKey{"E", "1.0"}: &Module{
			Key:  ModuleKey{"E", "1.0"},
			Deps: map[string]ModuleKey{},
		},
	}
	ctx := &context{
		rootModuleName: "A",
		depGraph:       depGraph,
		overrideSet:    OverrideSet{},
	}
	require.NoError(t, Selection(ctx))
	expectedDepGraph := DepGraph{
		ModuleKey{"A", ""}: &Module{
			Key: ModuleKey{"A", "1.0"},
			Deps: map[string]ModuleKey{
				"myB": {"B", ""},
				"myC": {"C", "1.0"},
			},
		},
		ModuleKey{"B", ""}: &Module{
			Key: ModuleKey{"B", "local-version"},
			Deps: map[string]ModuleKey{
				"myD": {"D", "1.1"},
			},
		},
		ModuleKey{"C", "1.0"}: &Module{
			Key: ModuleKey{"C", "1.0"},
			Deps: map[string]ModuleKey{
				"yourD": {"D", "1.1"},
			},
		},
		ModuleKey{"D", "1.1"}: &Module{
			Key:  ModuleKey{"D", "1.1"},
			Deps: map[string]ModuleKey{},
		},
	}
	assert.Equal(t, expectedDepGraph, depGraph)
}
