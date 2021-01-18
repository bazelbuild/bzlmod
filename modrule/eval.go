package modrule

import "go.starlark.net/starlark"

type Ruleset struct {
	ResolveFn starlark.Function
	FetchFn   starlark.Function
}

// GetRulesets executes the Starlark code contained in src and returns a map of all the Ruleset objects defined in the
// source.
func GetRulesets(src []byte) (map[string]*Ruleset, error) {
	panic("aaaahhhh")
}
