package modrule

import (
	"fmt"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

type Attr struct {
}

func (a *Attr) String() string {
	panic("implement me")
}

func (a *Attr) Type() string {
	panic("implement me")
}

func (a *Attr) Freeze() {
	panic("implement me")
}

func (a *Attr) Truth() starlark.Bool {
	panic("implement me")
}

func (a *Attr) Hash() (uint32, error) {
	panic("implement me")
}

func ExtractAttrMap(dict *starlark.Dict) (map[string]*Attr, error) {
	attrs := make(map[string]*Attr)
	for _, item := range dict.Items() {
		s, ok := starlark.AsString(item[0])
		if !ok {
			return nil, fmt.Errorf("got %v, want string", item[0].Type())
		}
		// TODO: Check that `s` is a valid identifier
		attr, ok := item[1].(*Attr)
		if !ok {
			return nil, fmt.Errorf("got %v, want Attr", item[1].Type())
		}
		attrs[s] = attr
	}
	return attrs, nil
}

var attrModule = &starlarkstruct.Module{
	Name:    "attr",
	Members: starlark.StringDict{},
}
