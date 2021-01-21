package modrule

import (
	"fmt"
	"go.starlark.net/starlark"
)

type Context struct{}

func (c Context) String() string        { return "ModuleRuleContext" }
func (c Context) Type() string          { return "ModuleRuleContext" }
func (c Context) Freeze()               {}
func (c Context) Truth() starlark.Bool  { return true }
func (c Context) Hash() (uint32, error) { return 0, fmt.Errorf("not hashable: ModuleRuleContext") }

func (c Context) Attr(name string) (starlark.Value, error) {
	panic("implement me")
}

func (c Context) AttrNames() []string {
	panic("implement me")
}
