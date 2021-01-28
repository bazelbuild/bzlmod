package modrule

import (
	"fmt"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"os"
	"runtime"
	"strings"
)

type Context struct {
	// Resolve only
	topModule starlark.Value // really *BazelModule

	// Fetch only
	repoName starlark.String
	repoInfo starlark.Value

	// Shared
	// An absolute filepath to the directory where all download, extract, template, etc paths are rooted.
	// Not called "repoPath" because during resolution, there is no repo yet.
	rootPath string
}

func NewResolveContext(topModule *BazelModule) *Context {
	// TODO: rootPath
	return &Context{
		topModule: topModule,
		repoInfo:  starlark.None,
		rootPath:  "",
	}
}

func NewFetchContext(repoName string, repoInfo starlark.Value, repoPath string) *Context {
	return &Context{
		topModule: starlark.None,
		repoName:  starlark.String(repoName),
		repoInfo:  repoInfo,
		rootPath:  repoPath,
	}
}

func (c *Context) String() string        { return "ModuleRuleContext" }
func (c *Context) Type() string          { return "ModuleRuleContext" }
func (c *Context) Freeze()               {}
func (c *Context) Truth() starlark.Bool  { return true }
func (c *Context) Hash() (uint32, error) { return 0, fmt.Errorf("not hashable: ModuleRuleContext") }

func buildEnvironDict() *starlark.Dict {
	environ := os.Environ()
	dict := starlark.NewDict(len(environ))
	for _, pair := range environ {
		s := strings.SplitN(pair, "=", 2)
		_ = dict.SetKey(starlark.String(s[0]), starlark.String(s[1]))
	}
	return dict
}

func getOSName() starlark.String {
	// OK, this is ridiculous. We have to somehow match the Bazel implementation of this, which just calls
	// `System.getProperties("os.name").toLowerCase()`. According to the OpenJDK source code, the "os.name" property is:
	//  - "Windows $VERSION" on windows (https://github.com/openjdk/jdk/blob/9604ee82690f89320614b37bfef4178abc869777/src/java.base/windows/native/libjava/java_props_md.c#L478)
	//    - GOOS will just be "windows", so to maximize the chance of this working, let's put "windows 10".
	//  - "Mac OS X" on mac (https://github.com/openjdk/jdk/blob/30b9ff660f07433f918b279b9098ab38a466da93/src/java.base/macosx/native/libjava/java_props_macosx.c#L235)
	//    - GOOS will be "darwin", so this is an easy translation.
	//  - `uname -s` on unix-like EXCEPT mac (https://github.com/openjdk/jdk/blob/05a764f4ffb8030d6b768f2d362c388e5aabd92d/src/java.base/unix/native/libjava/java_props_md.c#L403)
	//    - GOOS will mostly match this. The systems where GOOS doesn't match `uname -s` are probably not supported by
	//      Bazel anyway.
	// Well! That at least gives us something workable.
	switch runtime.GOOS {
	case "windows":
		return "windows 10"
	case "darwin":
		return "mac os x"
	default:
		return starlark.String(runtime.GOOS)
	}
}

var osStruct = starlarkstruct.FromStringDict(starlark.String("repository_os"), starlark.StringDict{
	"environ": buildEnvironDict(),
	"name":    getOSName(),
})

var contextProps = map[string]func(c *Context) starlark.Value{
	"name":       func(c *Context) starlark.Value { return c.repoName },
	"os":         func(c *Context) starlark.Value { return osStruct },
	"repo_info":  func(c *Context) starlark.Value { return c.repoInfo }, // TODO: call this `attr`?
	"top_module": func(c *Context) starlark.Value { return c.topModule },
}

var contextMethods = map[string]*starlark.Builtin{
	"delete":               starlark.NewBuiltin("delete", context_delete),
	"download":             starlark.NewBuiltin("download", context_download),
	"download_and_extract": starlark.NewBuiltin("download_and_extract", context_download_and_extract),
	"execute":              starlark.NewBuiltin("execute", context_execute),
	"extract":              starlark.NewBuiltin("extract", context_extract),
	"file":                 starlark.NewBuiltin("file", context_file),
	"patch":                starlark.NewBuiltin("patch", context_patch),
	"path":                 starlark.NewBuiltin("path", context_path),
	"read":                 starlark.NewBuiltin("read", context_read),
	"report_progress":      starlark.NewBuiltin("report_progress", context_report_progress),
	"symlink":              starlark.NewBuiltin("symlink", context_symlink),
	"template":             starlark.NewBuiltin("template", context_template),
	"which":                starlark.NewBuiltin("which", context_which),
}

func (c *Context) Attr(name string) (starlark.Value, error) {
	if prop, ok := contextProps[name]; ok {
		return prop(c), nil
	}
	return methodAttr(name, c, contextMethods), nil
}

func (c *Context) AttrNames() []string {
	names := make([]string, 0, len(contextProps)+len(contextMethods))
	for name := range contextProps {
		names = append(names, name)
	}
	for name := range contextMethods {
		names = append(names, name)
	}
	return names
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

var bazelModuleProps = map[string]func(bm *BazelModule) starlark.Value{
	"name":           func(bm *BazelModule) starlark.Value { return bm.Name },
	"version":        func(bm *BazelModule) starlark.Value { return bm.Version },
	"bazel_deps":     func(bm *BazelModule) starlark.Value { return starlark.NewList(bm.BazelDeps) },
	"rule_instances": func(bm *BazelModule) starlark.Value { return bm.RuleInstances },
}

var bazelModuleMethods = map[string]*starlark.Builtin{
	"bfs": starlark.NewBuiltin("bfs", bazelModule_bfs),
}

func methodAttr(name string, receiver starlark.Value, builtins map[string]*starlark.Builtin) starlark.Value {
	builtin := builtins[name]
	if builtin == nil {
		return nil
	}
	return builtin.BindReceiver(receiver)
}

func (bm *BazelModule) Attr(name string) (starlark.Value, error) {
	if prop, ok := bazelModuleProps[name]; ok {
		return prop(bm), nil
	}
	return methodAttr(name, bm, bazelModuleMethods), nil
}

func (bm *BazelModule) AttrNames() []string {
	names := make([]string, 0, len(bazelModuleProps)+len(bazelModuleMethods))
	for name := range bazelModuleProps {
		names = append(names, name)
	}
	for name := range bazelModuleMethods {
		names = append(names, name)
	}
	return names
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

func bazelModule_bfs(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

func context_delete(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// TODO
	return starlark.None, nil
}

func context_download(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// TODO
	return starlark.None, nil
}

func context_download_and_extract(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// TODO
	return starlark.None, nil
}

func context_execute(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// TODO
	return starlark.None, nil
}

func context_extract(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// TODO
	return starlark.None, nil
}

func context_file(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// TODO
	return starlark.None, nil
}

func context_patch(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// TODO
	return starlark.None, nil
}

func context_path(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// TODO
	return starlark.None, nil
}

func context_read(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// TODO
	return starlark.None, nil
}

func context_report_progress(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// TODO
	return starlark.None, nil
}

func context_symlink(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// TODO
	return starlark.None, nil
}

func context_template(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// TODO
	return starlark.None, nil
}

func context_which(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// TODO
	return starlark.None, nil
}
