package modrule

import (
	"fmt"
	"go.starlark.net/starlark"
)

type Context struct {
	topModule *BazelModule
	repoName  string
	repoInfo  starlark.Value
}

func NewResolveContext(topModule *BazelModule) Context {
	return Context{topModule: topModule}
}

func NewFetchContext(repoName string, repoInfo starlark.Value) Context {
	return Context{repoName: repoName, repoInfo: repoInfo}
}

func (c Context) String() string        { return "ModuleRuleContext" }
func (c Context) Type() string          { return "ModuleRuleContext" }
func (c Context) Freeze()               {}
func (c Context) Truth() starlark.Bool  { return true }
func (c Context) Hash() (uint32, error) { return 0, fmt.Errorf("not hashable: ModuleRuleContext") }

func (c Context) Attr(name string) (starlark.Value, error) {
	// TODO: execute, top_module, os?, repo_name, repo_info
	panic("implement me")
}

func (c Context) AttrNames() []string {
	panic("implement me")
}

type BazelModule struct {
	Name    starlark.String
	Version starlark.String
	// BazelDeps is a list of BazelModule objects, each corresponding to a `bazel_dep` of the current BazelModule.
	// The type is []starlark.Value but is really []*BazelModule.
	BazelDeps     []starlark.Value
	RuleInstances *RuleInstances
}

func (bm *BazelModule) String() string        { return fmt.Sprintf("BazelModule[%v, ...]", string(bm.Name)) }
func (bm *BazelModule) Type() string          { return "BazelModule" }
func (bm *BazelModule) Freeze()               {}
func (bm *BazelModule) Truth() starlark.Bool  { return true }
func (bm *BazelModule) Hash() (uint32, error) { return 0, fmt.Errorf("not hashable: BazelModule") }

func (bm *BazelModule) Attr(name string) (starlark.Value, error) {
	switch name {
	case "name":
		return bm.Name, nil
	case "version":
		return bm.Version, nil
	case "bazel_deps":
		return starlark.NewList(bm.BazelDeps), nil
	case "rule_instances":
		return bm.RuleInstances, nil
	case "bfs":
		return bfsBuiltin.BindReceiver(bm), nil
	default:
		return nil, nil
	}
}

func (bm *BazelModule) AttrNames() []string {
	return []string{"name", "version", "bazel_deps", "rule_instances", "bfs"}
}

type RuleInstances struct {
	// inst is a mapping from rule name (more precisely ruleset member name) to a list of instances (aka Tags) of that
	// rule. The value type of the map here is []starlark.Value but is effectively []*RuleInstance.
	inst map[string][]starlark.Value
}

func NewRuleInstances() *RuleInstances {
	return &RuleInstances{inst: make(map[string][]starlark.Value)}
}

func (ri *RuleInstances) Append(ruleName string, instance *RuleInstance) {
	ri.inst[ruleName] = append(ri.inst[ruleName], instance)
}

func (ri *RuleInstances) String() string        { return "RuleInstances[...]" }
func (ri *RuleInstances) Type() string          { return "RuleInstances" }
func (ri *RuleInstances) Truth() starlark.Bool  { return true }
func (ri *RuleInstances) Hash() (uint32, error) { return 0, fmt.Errorf("not hashable: RuleInstances") }
func (ri *RuleInstances) Freeze()               {}

func (ri *RuleInstances) Attr(name string) (starlark.Value, error) {
	return starlark.NewList(ri.inst[name]), nil
}

func (ri *RuleInstances) AttrNames() []string {
	keys := make([]string, len(ri.inst))
	i := 0
	for key := range ri.inst {
		keys[i] = key
		i++
	}
	return keys
}

func bfsFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var callback starlark.Callable
	err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &callback)
	if err != nil {
		return nil, err
	}
	mod := b.Receiver().(*BazelModule)
	queue := []*BazelModule{mod}
	discovered := map[*BazelModule]bool{mod: true}
	for len(queue) > 0 {
		mod = queue[0]
		queue = queue[1:]
		_, err = starlark.Call(t, callback, []starlark.Value{mod}, nil)
		if err != nil {
			return nil, err
		}
		for _, dep := range mod.BazelDeps {
			dep := dep.(*BazelModule)
			if !discovered[dep] {
				discovered[dep] = true
				queue = append(queue, dep)
			}
		}
	}
	return starlark.None, nil
}

var bfsBuiltin = starlark.NewBuiltin("bfs", bfsFn)
