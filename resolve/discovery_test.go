package resolve

import (
  "fmt"
  "io/ioutil"
  "path/filepath"
  "testing"
)

type myReg struct {}

func (r *myReg) GetModuleBazel(name string, version string, registry string) ([]byte, error) {
  if name == "B" && version == "1.0" {
    return []byte(`module(name="B"); bazel_dep(name="D", version="0.1")`), nil
  } else if name == "C" && version == "2.0" {
    return []byte(`module(name="C"); bazel_dep(name="D", version="0.1")`), nil
  } else if name == "D" && version == "0.1" {
    return []byte(`module(name="D")`), nil
  }
  return nil, fmt.Errorf("bad key: name = %v, version = %v", name, version)
}

func TestSimpleDiamond(t *testing.T) {
  wsDir := t.TempDir()
  moduleBazel := []byte(`
module(name="A")
bazel_dep(name="B", version="1.0")
bazel_dep(name="C", version="2.0")
`)
  if err := ioutil.WriteFile(filepath.Join(wsDir, "MODULE.bazel"), moduleBazel, 0644); err != nil {
    t.Fatal(err)
  }
  reg := &myReg{}
  var v DiscoveryResult
  var err error
  if v, err = Discovery(wsDir, reg); err != nil {
    t.Fatal(err)
  }
  t.Log(len(v.DepGraph))
  t.Log("all good")
}
