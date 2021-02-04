package fetch

import (
	"io/ioutil"
	"path/filepath"
)

func VerifyFingerprintFile(dir string, fprint string) bool {
	actualFprint, err := ioutil.ReadFile(filepath.Join(dir, "bzlmod.fingerprint"))
	return err == nil && string(actualFprint) == fprint
}

func WriteFingerprintFile(dir string, fprint string) error {
	return ioutil.WriteFile(filepath.Join(dir, "bzlmod.fingerprint"), []byte(fprint), 0664)
}
