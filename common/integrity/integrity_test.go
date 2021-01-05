package integrity

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

var payload = []byte(`alert('Hello, world.');`)
var badPayload = []byte(`aLeRt('heLlO, wOrlD.');`)

func TestEachAlgo(t *testing.T) {
	sha256Hash := sha256.Sum256(payload)
	sha384Hash := sha512.Sum384(payload)
	sha512Hash := sha512.Sum512(payload)
	tests := []struct {
		algo string
		hash []byte
	}{
		{"sha256", sha256Hash[:]},
		{"sha384", sha384Hash[:]},
		{"sha512", sha512Hash[:]},
	}
	for _, test := range tests {
		integrity := fmt.Sprintf("%v-%v", test.algo, base64.StdEncoding.EncodeToString(test.hash))
		success, err := Check(bytes.NewReader(payload), integrity)
		if assert.NoError(t, err, test.algo+" matches") {
			assert.True(t, success, test.algo+" matches")
		}
		success, err = Check(bytes.NewReader(badPayload), integrity)
		if assert.NoError(t, err, test.algo+" doesn't match") {
			assert.False(t, success, test.algo+" doesn't match")
		}
	}
}

func TestComplexIntegrity(t *testing.T) {
	sha512Hash := sha512.Sum512(payload)
	sha256Hash := sha256.Sum256(badPayload) // purposely use a wrong hash for a lower-priority algorithm

	integrity := fmt.Sprintf(
		"sha256-%v weirdalgo-%v \nsha512-%v?some-random-options md5-%v ",
		base64.StdEncoding.EncodeToString(sha256Hash[:]),
		base64.StdEncoding.EncodeToString([]byte("some random digest")),
		base64.StdEncoding.EncodeToString(sha512Hash[:]),
		base64.StdEncoding.EncodeToString([]byte("some other random digest")),
	)
	success, err := Check(bytes.NewReader(payload), integrity)
	if assert.NoError(t, err) {
		assert.True(t, success)
	}

	success, err = Check(bytes.NewReader(badPayload), integrity)
	if assert.NoError(t, err) {
		assert.False(t, success)
	}
}

func TestComplexIntegrity_MultipleDigests(t *testing.T) {
	goodHash := sha512.Sum512(payload)
	badHash := sha512.Sum512(badPayload) // this is called "badHash" but it's really "another good hash"

	integrity := fmt.Sprintf(
		"   sha512-%v  sha512-%v?kek ",
		base64.StdEncoding.EncodeToString(goodHash[:]),
		base64.StdEncoding.EncodeToString(badHash[:]),
	)
	success, err := Check(bytes.NewReader(payload), integrity)
	if assert.NoError(t, err) {
		assert.True(t, success)
	}

	success, err = Check(bytes.NewReader(badPayload), integrity)
	if assert.NoError(t, err) {
		assert.True(t, success)
	}

	success, err = Check(bytes.NewReader([]byte("eyyyy")), integrity)
	if assert.NoError(t, err) {
		// Only fail when none of the provided digests match.
		assert.False(t, success)
	}
}

func TestNoOpIntegrity(t *testing.T) {
	success, err := Check(bytes.NewReader(payload), "")
	if assert.NoError(t, err) {
		assert.True(t, success)
	}
	success, err = Check(bytes.NewReader(payload), "weirdalgo-"+base64.StdEncoding.EncodeToString([]byte("lol")))
	if assert.NoError(t, err) {
		assert.True(t, success)
	}
}

func TestCheckerWrite(t *testing.T) {
	sha512Hash := sha512.Sum512([]byte("this is an example"))
	checker, err := NewChecker("sha512-" + base64.StdEncoding.EncodeToString(sha512Hash[:]))
	if assert.NoError(t, err) {
		_, _ = checker.Write([]byte("this is a"))
		_, _ = checker.Write([]byte("n exa"))
		_, _ = checker.Write([]byte("mple"))
		assert.True(t, checker.Check())
	}
}

func TestBadIntegrity(t *testing.T) {
	_, err := NewChecker("sha512")
	if err == nil {
		t.Errorf("parse somehow succeeded for malformed integrity")
	}
	_, err = NewChecker("sha512-invalid:base64@payload")
	if err == nil {
		t.Errorf("parse somehow succeeded for invalid base64 payload")
	}
	_, err = NewChecker(fmt.Sprintf(
		"md5-%v  sha1-%v",
		base64.StdEncoding.EncodeToString([]byte("md5")),
		base64.StdEncoding.EncodeToString([]byte("sha1")),
	))
	if err == nil {
		t.Errorf("parse somehow succeeded for integrity with only deprecated algorithms")
	}
}
