package modrule

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/bazelbuild/bzlmod/common"
	integrities "github.com/bazelbuild/bzlmod/common/integrity"
	"github.com/bazelbuild/bzlmod/fetch"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// BazelModule is a Starlark type that exposes the dependency graph to a module rule's resolve_fn. It replicates the
// dependency subgraph consisting of Bazel modules (hence the name) and the pertinent "tags" (RuleInstances) on each
// module. It's exposed through the ModuleRuleContext object.
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

// RuleInstances is a Starlark object that effectively acts as a struct, where each field is a list of module rule
// invocations (RuleInstance). It's exposed through the BazelModule object.
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

// RuleInstance is a Starlark object that represents a single module rule invocation. It's exposed through the
// RuleInstances object.
type RuleInstance struct {
	Rule  *Rule
	Attrs map[string]starlark.Value
}

func NewRuleInstance(r *Rule, kwargs []starlark.Tuple) (*RuleInstance, error) {
	inst, err := InstantiateAttrs(r.Attrs, kwargs)
	if err != nil {
		return nil, err
	}
	return &RuleInstance{
		Rule:  r,
		Attrs: inst,
	}, nil
}

func (ri *RuleInstance) String() string        { return "RuleInstance[...]" }
func (ri *RuleInstance) Type() string          { return "RuleInstance" }
func (ri *RuleInstance) Truth() starlark.Bool  { return true }
func (ri *RuleInstance) Hash() (uint32, error) { return 0, fmt.Errorf("not hashable: RuleInstance") }

func (ri *RuleInstance) Freeze() {
	for _, attr := range ri.Attrs {
		attr.Freeze()
	}
}

func (ri *RuleInstance) Attr(name string) (starlark.Value, error) {
	return ri.Attrs[name], nil
}

func (ri *RuleInstance) AttrNames() []string {
	keys := make([]string, len(ri.Attrs))
	i := 0
	for key := range ri.Attrs {
		keys[i] = key
		i++
	}
	return keys
}

// Path is an absolute path to a file. It's created by the ModuleRuleContext.path method.
// Technically we should match the Java implementation but it is quite a mess
// (https://cs.opensource.google/bazel/bazel/+/master:src/main/java/com/google/devtools/build/lib/vfs/Path.java), so we
// just use Go's filepath module.
type Path string

func (p Path) String() string        { return string(p) }
func (p Path) Type() string          { return "path" }
func (p Path) Freeze()               {}
func (p Path) Truth() starlark.Bool  { return len(p) > 0 }
func (p Path) Hash() (uint32, error) { return starlark.String(p).Hash() }

func path_exists(p Path) (starlark.Value, error) {
	_, err := os.Stat(string(p))
	if err == nil {
		return starlark.True, nil
	}
	if os.IsNotExist(err) {
		return starlark.False, nil
	}
	return nil, err
}

func path_realpath(p Path) (starlark.Value, error) {
	rp, err := filepath.EvalSymlinks(string(p))
	if err != nil {
		return nil, err
	}
	return Path(rp), err
}

var pathProps = map[string]func(p Path) (starlark.Value, error){
	"basename": func(p Path) (starlark.Value, error) { return starlark.String(filepath.Base(string(p))), nil },
	"dirname":  func(p Path) (starlark.Value, error) { return Path(filepath.Dir(string(p))), nil },
	"exists":   path_exists,
	"realpath": path_realpath,
}

var pathMethods = map[string]*starlark.Builtin{
	"get_child": starlark.NewBuiltin("get_child", path_get_child),
	"readdir":   starlark.NewBuiltin("readdir", path_readdir),
}

func (p Path) Attr(name string) (starlark.Value, error) {
	if prop, ok := pathProps[name]; ok {
		return prop(p)
	}
	return methodAttr(name, p, pathMethods), nil
}

func (p Path) AttrNames() []string {
	names := make([]string, 0, len(pathProps)+len(pathMethods))
	for name := range pathProps {
		names = append(names, name)
	}
	for name := range pathMethods {
		names = append(names, name)
	}
	return names
}

func path_get_child(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var childPath starlark.String
	err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &childPath)
	if err != nil {
		return nil, err
	}
	p := b.Receiver().(Path)
	return Path(filepath.Join(string(p), string(childPath))), nil
}

func path_readdir(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0)
	if err != nil {
		return nil, err
	}
	p := string(b.Receiver().(Path))
	files, err := ioutil.ReadDir(p)
	if err != nil {
		return nil, err
	}
	dirents := make([]starlark.Value, 0, len(files))
	for _, file := range files {
		dirents = append(dirents, Path(filepath.Join(p, file.Name())))
	}
	list := starlark.NewList(dirents)
	list.Freeze()
	return list, nil
}

// Context (really ModuleRuleContext) is a Starlark object that provides module rules' resolve_fn and fetch_fn with
// information and a means of performing non-hermetic operations (such as downloads and executing arbitrary binaries).
type Context struct {
	// Resolve only
	topModule starlark.Value // really *BazelModule

	// Fetch only
	repoName starlark.String
	repoInfo starlark.Value

	// Shared
	// An absolute filepath to the directory where all download, extract, template, etc paths are rooted.
	// Not called "repoPath" because during resolution, there is no repo yet.
	rootPath      string
	labelResolver common.LabelResolver
}

func NewResolveContext(topModule *BazelModule) *Context {
	// TODO: Fix up call site and populate all values
	return &Context{
		topModule: topModule,
		repoInfo:  starlark.None,
		rootPath:  "",
	}
}

func NewFetchContext(repoName string, repoInfo starlark.Value, repoPath string) *Context {
	// TODO: Fix up call site and populate all values
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

// resolvePath resolves the given string, path, or label to an absolute file path. If the string is a relative path,
// it's relative the the context root path.
func (c *Context) resolvePath(v starlark.Value) (string, error) {
	if v == nil {
		v = starlark.String("")
	}
	switch vt := v.(type) {
	case starlark.String:
		return common.NormalizePath(c.rootPath, string(vt)), nil
	case Path:
		// Paths are always absolute.
		return string(vt), nil
	case *Label:
		// TODO: "curRepo" here requires special attention. It'll probably always be the module where the rule is
		//  declared, so `c.repoName` isn't going to cut it (since it's empty during resolution; and during fetch, it's
		//  the repo being fetched, although it has mostly the same repoDeps as the module rule declarer).
		result, err := c.labelResolver.ResolveLabel(string(c.repoName), "", (*common.Label)(vt))
		if err != nil {
			return "", err
		}
		return result.Filename, nil
	default:
		return "", fmt.Errorf("expected string, path, or Label, got %v", v.Type())
	}
}

func context_delete(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.Value
	err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &path)
	if err != nil {
		return nil, err
	}
	c := b.Receiver().(*Context)
	resolvedPath, err := c.resolvePath(path)
	if err != nil {
		return nil, err
	}
	err = os.Remove(resolvedPath)
	// It's amazingly difficult to deduce from the Bazel implementation just *when* this is supposed to return false.
	// From what I can tell, the return value should be true when there's no error (obviously), and false when the file
	// or directory doesn't exist in the first place. In all other error cases, an actual exception is thrown.
	if err == nil {
		return starlark.True, nil
	}
	if os.IsNotExist(err) {
		return starlark.False, nil
	}
	return nil, err
}

// Utility type to help unpack values of type "string or iterable of strings" often used by the url parameter of methods
// download and download_and_extract.
type stringOrStrings struct {
	slice []string
}

func (ss *stringOrStrings) Unpack(v starlark.Value) error {
	switch vt := v.(type) {
	case starlark.String:
		ss.slice = []string{string(vt)}
	case starlark.Iterable:
		var elem starlark.Value
		i := 0
		for it := vt.Iterate(); it.Next(&elem); i++ {
			s, ok := elem.(starlark.String)
			if !ok {
				return fmt.Errorf("element #%v is not string, but %v", i, elem.Type())
			}
			ss.slice = append(ss.slice, string(s))
		}
	default:
		return fmt.Errorf("want string or Iterable of strings, got %v", v.Type())
	}
	return nil
}

func sha256Integrity(sha256 string) (string, error) {
	p, err := hex.DecodeString(sha256)
	if err != nil {
		return "", err
	}
	return integrities.GenerateFromSha256(p), nil
}

func context_download(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		url         stringOrStrings
		output      starlark.Value
		sha256      string
		executable  = false
		allowFail   bool                  // TODO: use
		canonicalId string                // TODO: use
		auth        = starlark.NewDict(0) // TODO: use
		integrity   string
	)
	err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"url", &url,
		"output?", &output,
		"sha256?", &sha256,
		"executable?", &executable,
		"allow_fail?", &allowFail,
		"canonical_id?", &canonicalId,
		"auth?", &auth,
		"integrity?", &integrity,
	)
	if err != nil {
		return nil, err
	}
	c := b.Receiver().(*Context)
	outputPath, err := c.resolvePath(output)
	if err != nil {
		return nil, fmt.Errorf("invalid output path: %v", err)
	}
	if integrity == "" {
		integrity, err = sha256Integrity(sha256)
		if err != nil {
			return nil, fmt.Errorf("invalid sha256: %v", err)
		}
	}
	result, err := fetch.Download(url.slice, integrity)
	if err != nil {
		return nil, err
	}
	// Copy the file from the download cache to the output path.
	src, err := os.Open(result.Filename)
	if err != nil {
		return nil, fmt.Errorf("error opening downloaded file for reading: %v", err)
	}
	defer src.Close()
	err = os.MkdirAll(filepath.Dir(outputPath), 0775)
	if err != nil {
		return nil, err
	}
	var dstPerm os.FileMode = 0664
	if executable {
		dstPerm = 0775
	}
	dst, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, dstPerm)
	if err != nil {
		return nil, fmt.Errorf("error opening target file for writing: %v", err)
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	if err != nil {
		return nil, fmt.Errorf("error copying file to output path: %v", err)
	}
	returnVal := starlarkstruct.FromKeywords(starlarkstruct.Default, []starlark.Tuple{
		{starlark.String("sha256"), starlark.String(hex.EncodeToString(result.Sha256))},
		{starlark.String("integrity"), starlark.String(integrities.GenerateFromSha256(result.Sha256))},
	})
	return returnVal, nil
}

func context_download_and_extract(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		url         stringOrStrings
		output      starlark.Value
		sha256      string
		archiveType string // TODO: use
		stripPrefix string
		allowFail   bool                  // TODO: use
		canonicalId string                // TODO: use
		auth        = starlark.NewDict(0) // TODO: use
		integrity   string
	)
	err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"url", &url,
		"output?", &output,
		"sha256?", &sha256,
		"type?", &archiveType,
		"stripPrefix?", &stripPrefix,
		"allow_fail?", &allowFail,
		"canonical_id?", &canonicalId,
		"auth?", &auth,
		"integrity?", &integrity,
	)
	if err != nil {
		return nil, err
	}
	c := b.Receiver().(*Context)
	outputPath, err := c.resolvePath(output)
	if err != nil {
		return nil, fmt.Errorf("invalid output path: %v", err)
	}
	if integrity == "" {
		integrity, err = sha256Integrity(sha256)
		if err != nil {
			return nil, fmt.Errorf("invalid sha256: %v", err)
		}
	}
	result, err := fetch.Download(url.slice, integrity)
	if err != nil {
		return nil, err
	}
	err = fetch.Extract(result.Filename, outputPath, stripPrefix)
	if err != nil {
		return nil, err
	}
	returnVal := starlarkstruct.FromKeywords(starlarkstruct.Default, []starlark.Tuple{
		{starlark.String("sha256"), starlark.String(hex.EncodeToString(result.Sha256))},
		{starlark.String("integrity"), starlark.String(integrities.GenerateFromSha256(result.Sha256))},
	})
	return returnVal, nil
}

func context_execute(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		arguments  *starlark.List
		timeout    = 600
		environ    = starlark.NewDict(0)
		quiet      = true
		workingDir string
	)
	err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"arguments", &arguments,
		"timeout?", &timeout,
		"environ?", &environ,
		"quiet?", &quiet,
		"working_directory?", &workingDir,
	)
	if err != nil {
		return nil, err
	}

	// Argument validation
	if arguments.Len() == 0 {
		return nil, fmt.Errorf("empty command")
	}
	argumentPaths := make([]string, arguments.Len())
	c := b.Receiver().(*Context)
	for i := 0; i < arguments.Len(); i++ {
		argumentPaths[i], err = c.resolvePath(arguments.Index(i))
		if err != nil {
			return nil, fmt.Errorf("in argument #%v: %v", i, err)
		}
	}
	workingDirPath, err := c.resolvePath(starlark.String(workingDir))
	if err != nil {
		return nil, fmt.Errorf("in working_directory: %v", err)
	}
	env := os.Environ()
	for _, entry := range environ.Items() {
		key, ok := starlark.AsString(entry[0])
		if !ok {
			return nil, fmt.Errorf("want string keys for environ, got %v", entry[0].Type())
		}
		val, ok := starlark.AsString(entry[1])
		if !ok {
			return nil, fmt.Errorf("want string values for environ, got %v", entry[1].Type())
		}
		env = append(env, key+"="+val)
	}

	// Execute!
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argumentPaths[0], argumentPaths[1:]...)
	cmd.Env = env
	cmd.Dir = workingDirPath
	var stdout, stderr bytes.Buffer
	if quiet {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	} else {
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	}
	err = cmd.Run()
	returnCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			returnCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("couldn't run subprocess: %v", err)
		}
	}

	result := starlarkstruct.FromKeywords(starlark.String("exec_result"), []starlark.Tuple{
		{starlark.String("return_code"), starlark.MakeInt(returnCode)},
		{starlark.String("stdout"), starlark.String(stdout.Bytes())},
		{starlark.String("stderr"), starlark.String(stderr.Bytes())},
	})
	return result, nil
}

func context_extract(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		archive     starlark.Value
		output      starlark.Value
		stripPrefix string
	)
	err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"archive", &archive,
		"output?", &output,
		"stripPrefix?", &stripPrefix,
	)
	if err != nil {
		return nil, err
	}
	c := b.Receiver().(*Context)
	archivePath, err := c.resolvePath(archive)
	if err != nil {
		return nil, fmt.Errorf("invalid archive path: %v", err)
	}
	outputPath, err := c.resolvePath(output)
	if err != nil {
		return nil, fmt.Errorf("invalid output path: %v", err)
	}
	err = fetch.Extract(archivePath, outputPath, stripPrefix)
	if err != nil {
		return nil, err
	}
	return starlark.None, nil
}

func context_file(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		path       starlark.Value
		content    string
		executable = true
	)
	err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"path", &path,
		"content?", &content,
		"executable?", &executable,
	)
	if err != nil {
		return nil, err
	}
	c := b.Receiver().(*Context)
	resolvedPath, err := c.resolvePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %v", err)
	}
	err = os.MkdirAll(filepath.Dir(resolvedPath), 0775)
	if err != nil {
		return nil, err
	}
	var perm os.FileMode = 0664
	if executable {
		perm = 0775
	}
	err = ioutil.WriteFile(resolvedPath, []byte(content), perm)
	if err != nil {
		return nil, err
	}
	return starlark.None, nil
}

func context_patch(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		patchFile starlark.Value
		strip     = 0
	)
	err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"patch_file", &patchFile,
		"strip?", &strip,
	)
	if err != nil {
		return nil, err
	}
	c := b.Receiver().(*Context)
	patchFilePath, err := c.resolvePath(patchFile)
	if err != nil {
		return nil, fmt.Errorf("invalid patch_file path: %v", err)
	}
	patch := fetch.Patch{
		PatchFile:  patchFilePath,
		PatchStrip: strip,
	}
	err = patch.Apply(c.rootPath)
	if err != nil {
		return nil, err
	}
	return starlark.None, nil
}

func context_path(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.Value
	err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &path)
	if err != nil {
		return nil, err
	}
	c := b.Receiver().(*Context)
	resolvedPath, err := c.resolvePath(path)
	if err != nil {
		return nil, err
	}
	return Path(resolvedPath), nil
}

func context_read(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.Value
	err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &path)
	if err != nil {
		return nil, err
	}
	c := b.Receiver().(*Context)
	resolvedPath, err := c.resolvePath(path)
	if err != nil {
		return nil, err
	}
	content, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return nil, err
	}
	return starlark.String(content), nil
}

func context_report_progress(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var status string
	err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &status)
	if err != nil {
		return nil, err
	}
	// Not sure what to do with this... Let's just print it out for now.
	fmt.Println(t.Name, ": ", status)
	return starlark.None, nil
}

func context_symlink(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var from, to starlark.Value
	err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 2, &from, &to)
	if err != nil {
		return nil, err
	}
	c := b.Receiver().(*Context)
	fromPath, err := c.resolvePath(from)
	if err != nil {
		return nil, fmt.Errorf("invalid from path: %v", err)
	}
	toPath, err := c.resolvePath(to)
	if err != nil {
		return nil, fmt.Errorf("invalid to path: %v", err)
	}
	err = os.MkdirAll(filepath.Dir(toPath), 0775)
	if err != nil {
		return nil, err
	}
	err = os.Symlink(fromPath, toPath)
	if err != nil {
		return nil, err
	}
	return starlark.None, nil
}

func context_template(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		file       starlark.Value
		template   starlark.Value
		subst      = starlark.NewDict(0)
		executable = true
	)
	err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"path", &file,
		"template", &template,
		"substitutions?", &subst,
		"executable?", &executable,
	)
	if err != nil {
		return nil, err
	}
	c := b.Receiver().(*Context)
	filePath, err := c.resolvePath(file)
	if err != nil {
		return nil, fmt.Errorf("invalid file path: %v", err)
	}
	templatePath, err := c.resolvePath(template)
	if err != nil {
		return nil, fmt.Errorf("invalid template path: %v", err)
	}
	contents, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return nil, err
	}
	for _, entry := range subst.Items() {
		key, ok := starlark.AsString(entry[0])
		if !ok {
			return nil, fmt.Errorf("want string keys for substitutions, got %v", entry[0].Type())
		}
		val, ok := starlark.AsString(entry[1])
		if !ok {
			return nil, fmt.Errorf("want string values for substitutions, got %v", entry[1].Type())
		}
		contents = bytes.ReplaceAll(contents, []byte(key), []byte(val))
	}
	err = os.MkdirAll(filepath.Dir(filePath), 0775)
	if err != nil {
		return nil, err
	}
	var perm os.FileMode = 0664
	if executable {
		perm = 0775
	}
	err = ioutil.WriteFile(filePath, contents, perm)
	if err != nil {
		return nil, err
	}
	return starlark.None, nil
}

func context_which(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var program string
	err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &program)
	if err != nil {
		return nil, err
	}
	programPath, err := exec.LookPath(program)
	if err != nil {
		return nil, err
	}
	absPath, err := filepath.Abs(programPath)
	if err != nil {
		return nil, err
	}
	return Path(absPath), nil
}
