package modrule

import (
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

const tagsKey = "tags"

// BazelDep is a Starlark value that's returned by a `bazel_dep` directive in the MODULE.bazel file.
type BazelDep struct {
	moduleKey common.ModuleKey
}

func NewBazelDep(moduleKey common.ModuleKey) *BazelDep {
	return &BazelDep{moduleKey}
}

func (b *BazelDep) String() string        { return fmt.Sprintf("bazel_dep(%v)", b.moduleKey) }
func (b *BazelDep) Type() string          { return "bazel_dep" }
func (b *BazelDep) Freeze()               {}
func (b *BazelDep) Truth() starlark.Bool  { return true }
func (b *BazelDep) Hash() (uint32, error) { return 0, fmt.Errorf("not hashable: bazel_dep") }

func (b *BazelDep) Attr(name string) (starlark.Value, error) {
	return &rulesetCallable{b.moduleKey, name}, nil
}

func (b *BazelDep) AttrNames() []string {
	// As this is a "smart object", we don't know what attributes it has.
	return nil
}

// rulesetCallable is a Starlark value that's returned by an attribute of a BazelDep object.
type rulesetCallable struct {
	moduleKey   common.ModuleKey
	rulesetName string
}

func (rs *rulesetCallable) String() string {
	return fmt.Sprintf("ruleset_callable(%v.%v)", rs.moduleKey, rs.rulesetName)
}
func (rs *rulesetCallable) Type() string         { return "ruleset_callable" }
func (rs *rulesetCallable) Freeze()              {}
func (rs *rulesetCallable) Truth() starlark.Bool { return true }
func (rs *rulesetCallable) Hash() (uint32, error) {
	return 0, fmt.Errorf("not hashable: ruleset_callable")
}

func (rs *rulesetCallable) Attr(name string) (starlark.Value, error) {
	return &ruleCallable{rs.moduleKey, rs.rulesetName, name}, nil
}

func (rs *rulesetCallable) AttrNames() []string {
	// As this is a "smart object", we don't know what attributes it has.
	return nil
}

func (rs *rulesetCallable) Name() string {
	return rs.String()
}

func (rs *rulesetCallable) CallInternal(thread *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// A rulesetCallable "key.rulesetName", when called, acts as if the ruleCallable "key.rulesetName.rulesetName" is
	// called.
	return (&ruleCallable{rs.moduleKey, rs.rulesetName, rs.rulesetName}).CallInternal(thread, args, kwargs)
}

type ruleCallable struct {
	moduleKey   common.ModuleKey
	rulesetName string
	ruleName    string
}

func (r *ruleCallable) String() string {
	return fmt.Sprintf("rule_callable(%v.%v.%v)", r.moduleKey, r.rulesetName, r.ruleName)
}
func (r *ruleCallable) Type() string          { return "rule_callable" }
func (r *ruleCallable) Freeze()               {}
func (r *ruleCallable) Truth() starlark.Bool  { return true }
func (r *ruleCallable) Hash() (uint32, error) { return 0, fmt.Errorf("not hashable: rule_callable") }
func (r *ruleCallable) Name() string          { return r.String() }

func (r *ruleCallable) CallInternal(thread *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	thread.SetLocal(tagsKey, append(GetTags(thread), Tag{
		ModuleKey:   r.moduleKey,
		RulesetName: r.rulesetName,
		RuleName:    r.ruleName,
		Args:        args,
		Kwargs:      kwargs,
		Pos:         thread.CallFrame(0).Pos,
	}))
	return starlark.None, nil
}

// Tag is an invocation of a module rule, or more specifically, a function call in the MODULE.bazel file that looks like
//   $id.$ruleset.$rule($args...)
// where $id is the return value of a bazel_dep() or module() invocation.
type Tag struct {
	ModuleKey   common.ModuleKey
	RulesetName string
	RuleName    string
	Args        starlark.Tuple
	Kwargs      []starlark.Tuple
	Pos         syntax.Position
}

// Retrieves all the Tags of a module, after said module's MODULE.bazel file has been executed on the given thread.
func GetTags(thread *starlark.Thread) []Tag {
	return thread.Local(tagsKey).([]Tag)
}
