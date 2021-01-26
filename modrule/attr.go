package modrule

import (
	"fmt"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

type AttrType int8

const (
	AttrBool AttrType = iota
	AttrInt
	AttrIntList
	AttrString
	AttrStringDict
	AttrStringList
	AttrStringListDict
)

type Attr struct {
	// AttrType describes the type of the attribute.
	AttrType AttrType
	// Doc is the documentation for this attribute.
	Doc string
	// Mandatory is whether this attribute must be specified.
	Mandatory bool
	// Default is the default value of this attribute.
	Default starlark.Value
	// Values denotes the list of possible values this attribute can have. Can be nil, meaning that all values are
	// possible.
	Values []starlark.Value
	// AllowEmpty is whether this attribute can have an empty value. Only relevant for List and Dict types.
	AllowEmpty bool
}

func (a *Attr) String() string        { return "Attr[...]" }
func (a *Attr) Type() string          { return "Attr" }
func (a *Attr) Freeze()               {}
func (a *Attr) Truth() starlark.Bool  { return true }
func (a *Attr) Hash() (uint32, error) { return 0, fmt.Errorf("not hashable: Attr") }

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

func isValueAllowed(v starlark.Value, allowedValues []starlark.Value) bool {
	for _, allowed := range allowedValues {
		eq, _ := starlark.Equal(v, allowed)
		if eq {
			return true
		}
	}
	return false
}

func validateOneAttr(attr *Attr, name string, value starlark.Value) error {
	switch attr.AttrType {
	case AttrBool:
		if _, ok := value.(starlark.Bool); !ok {
			return fmt.Errorf("for attr %q, expected bool, got %v", name, value.Type())
		}
	case AttrInt:
		i, ok := value.(starlark.Int)
		if !ok {
			return fmt.Errorf("for attr %q, expected int, got %v", name, value.Type())
		}
		if !isValueAllowed(i, attr.Values) {
			return fmt.Errorf("for attr %q, expected one of %v, got %v", name, attr.Values, i)
		}
	case AttrIntList:
		list, ok := value.(*starlark.List)
		if !ok {
			return fmt.Errorf("for attr %q, expected int list, got %v", name, value.Type())
		}
		if list.Len() == 0 && !attr.AllowEmpty {
			return fmt.Errorf("for attr %q which must not be empty, got empty list", name)
		}
		for i := 0; i < list.Len(); i++ {
			if _, ok = list.Index(i).(starlark.Int); !ok {
				return fmt.Errorf("for attr %q, expected int list, but element #%v is %v", name, i, list.Index(i).Type())
			}
		}
	case AttrString:
		s, ok := value.(starlark.String)
		if !ok {
			return fmt.Errorf("for attr %q, expected string, got %v", name, value.Type())
		}
		if !isValueAllowed(s, attr.Values) {
			return fmt.Errorf("for attr %q, expected one of %v, got %v", name, attr.Values, s)
		}
	case AttrStringDict:
		dict, ok := value.(*starlark.Dict)
		if !ok {
			return fmt.Errorf("for attr %q, expected string dict, got %v", name, value.Type())
		}
		if dict.Len() == 0 && !attr.AllowEmpty {
			return fmt.Errorf("for attr %q which must not be empty, got empty dict", name)
		}
		for _, elem := range dict.Items() {
			key, ok := starlark.AsString(elem[0])
			if !ok {
				return fmt.Errorf("for attr %q, expected string dict, got a key of type %v", name, elem[0].Type())
			}
			if _, ok = elem[1].(starlark.String); !ok {
				return fmt.Errorf("for attr %q, expected string dict, but element with key %q is %v", name, key, elem[1].Type())
			}
		}
	case AttrStringList:
		list, ok := value.(*starlark.List)
		if !ok {
			return fmt.Errorf("for attr %q, expected string list, got %v", name, value.Type())
		}
		if list.Len() == 0 && !attr.AllowEmpty {
			return fmt.Errorf("for attr %q which must not be empty, got empty list", name)
		}
		for i := 0; i < list.Len(); i++ {
			if _, ok = list.Index(i).(starlark.String); !ok {
				return fmt.Errorf("for attr %q, expected string list, but element #%v is %v", name, i, list.Index(i).Type())
			}
		}
	case AttrStringListDict:
		dict, ok := value.(*starlark.Dict)
		if !ok {
			return fmt.Errorf("for attr %q, expected string list dict, got %v", name, value.Type())
		}
		if dict.Len() == 0 && !attr.AllowEmpty {
			return fmt.Errorf("for attr %q which must not be empty, got empty dict", name)
		}
		for _, elem := range dict.Items() {
			key, ok := starlark.AsString(elem[0])
			if !ok {
				return fmt.Errorf("for attr %q, expected string list dict, got a key of type %v", name, elem[0].Type())
			}
			list, ok := elem[1].(*starlark.List)
			if !ok {
				return fmt.Errorf("for attr %q, expected string list dict, but element with key %q is %v", name, key, elem[1].Type())
			}
			for i := 0; i < list.Len(); i++ {
				if _, ok = list.Index(i).(starlark.String); !ok {
					return fmt.Errorf("for attr %q, expected string list dict, but list with key %q's element #%v is %v", name, key, i, list.Index(i).Type())
				}
			}
		}
	}
	return nil
}

func InstantiateAttrs(attrs map[string]*Attr, kwargs []starlark.Tuple) (map[string]starlark.Value, error) {
	inst := make(map[string]starlark.Value, len(attrs))
	for _, elem := range kwargs {
		name, arg := string(elem[0].(starlark.String)), elem[1]
		attr, ok := attrs[name]
		if !ok {
			return nil, fmt.Errorf("no attr named %q", name)
		}
		if err := validateOneAttr(attr, name, arg); err != nil {
			return nil, err
		}
		inst[name] = arg
	}
	for name, attr := range attrs {
		_, ok := inst[name]
		if ok {
			continue
		}
		if attr.Mandatory {
			return nil, fmt.Errorf("attr %q is mandatory", name)
		}
		inst[name] = attr.Default
	}
	return inst, nil
}

func listToValueSlice(list *starlark.List) []starlark.Value {
	slice := make([]starlark.Value, list.Len())
	for i := 0; i < list.Len(); i++ {
		slice[i] = list.Index(i)
	}
	return slice
}

func attrBoolFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	attr := &Attr{
		AttrType: AttrBool,
		Default:  starlark.False,
	}
	err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"default?", &attr.Default,
		"doc?", &attr.Doc,
		"mandatory?", &attr.Mandatory,
	)
	if err != nil {
		return nil, err
	}
	return attr, nil
}

func attrIntFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	var values *starlark.List
	attr := &Attr{
		AttrType: AttrInt,
		Default:  starlark.MakeInt(0),
	}
	err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"default?", &attr.Default,
		"doc?", &attr.Doc,
		"mandatory?", &attr.Mandatory,
		"values?", &values,
	)
	if err != nil {
		return nil, err
	}
	attr.Values = listToValueSlice(values)
	for i, v := range attr.Values {
		if _, ok := v.(starlark.Int); !ok {
			return nil, fmt.Errorf("list of allowed values contains non-int at #%v: %v", i, v)
		}
	}
	return attr, nil
}

func attrIntListFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	attr := &Attr{
		AttrType:   AttrIntList,
		Default:    starlark.NewList(nil),
		AllowEmpty: true,
	}
	err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"default?", &attr.Default,
		"doc?", &attr.Doc,
		"mandatory?", &attr.Mandatory,
		"allow_empty?", &attr.AllowEmpty,
	)
	if err != nil {
		return nil, err
	}
	return attr, nil
}

func attrStringFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	var values *starlark.List
	attr := &Attr{
		AttrType: AttrString,
		Default:  starlark.String(""),
	}
	err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"default?", &attr.Default,
		"doc?", &attr.Doc,
		"mandatory?", &attr.Mandatory,
		"values?", &values,
	)
	if err != nil {
		return nil, err
	}
	attr.Values = listToValueSlice(values)
	for i, v := range attr.Values {
		if _, ok := v.(starlark.String); !ok {
			return nil, fmt.Errorf("list of allowed values contains non-string at #%v: %v", i, v)
		}
	}
	return attr, nil
}

func attrStringDictFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	attr := &Attr{
		AttrType:   AttrStringDict,
		Default:    starlark.NewDict(0),
		AllowEmpty: true,
	}
	err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"default?", &attr.Default,
		"doc?", &attr.Doc,
		"mandatory?", &attr.Mandatory,
		"allow_empty?", &attr.AllowEmpty,
	)
	if err != nil {
		return nil, err
	}
	return attr, nil
}

func attrStringListFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	attr := &Attr{
		AttrType:   AttrStringList,
		Default:    starlark.NewList(nil),
		AllowEmpty: true,
	}
	err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"default?", &attr.Default,
		"doc?", &attr.Doc,
		"mandatory?", &attr.Mandatory,
		"allow_empty?", &attr.AllowEmpty,
	)
	if err != nil {
		return nil, err
	}
	return attr, nil
}

func attrStringListDictFn(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%v: unexpected positional arguments", b.Name())
	}
	attr := &Attr{
		AttrType:   AttrStringListDict,
		Default:    starlark.NewDict(0),
		AllowEmpty: true,
	}
	err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"default?", &attr.Default,
		"doc?", &attr.Doc,
		"mandatory?", &attr.Mandatory,
		"allow_empty?", &attr.AllowEmpty,
	)
	if err != nil {
		return nil, err
	}
	return attr, nil
}

var attrModule = &starlarkstruct.Module{
	Name: "attr",
	Members: starlark.StringDict{
		"bool":             starlark.NewBuiltin("attr.bool", attrBoolFn),
		"int":              starlark.NewBuiltin("attr.int", attrIntFn),
		"int_list":         starlark.NewBuiltin("attr.int_list", attrIntListFn),
		"string":           starlark.NewBuiltin("attr.string", attrStringFn),
		"string_dict":      starlark.NewBuiltin("attr.string_dict", attrStringDictFn),
		"string_list":      starlark.NewBuiltin("attr.string_list", attrStringListFn),
		"string_list_dict": starlark.NewBuiltin("attr.string_list_dict", attrStringListDictFn),
	},
}
