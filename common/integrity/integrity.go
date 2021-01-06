// An implementation of the Subresource Integrity W3C recommendation (http://www.w3.org/TR/SRI/)
package integrity

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"io"
	"regexp"
	"strings"
)

type algo struct {
	priority int
	fn       func() hash.Hash
}

// The standard specifies that there should be a function comparing the priority of arbitrary algorithms. We simplify
// this by just assigning a numerical priority to each algorithm (higher is better).
// A negative priority signifies a deprecated algorithm.
var algos = map[string]algo{
	"md5":    {-1, nil},
	"sha1":   {-1, nil},
	"sha256": {100, sha256.New},
	"sha384": {200, sha512.New384},
	"sha512": {300, sha512.New},
}

var exprRegexp = regexp.MustCompile(`^(\w+)-([\w+/]+={0,2})(\?.*)?$`)

var ErrBadIntegrity = errors.New("bad integrity metadata")

// Checks whether the contents of the reader matches the given integrity. This is simply a convenience wrapper around
// NewChecker and its methods.
func Check(r io.Reader, integrity string) (bool, error) {
	checker, err := NewChecker(integrity)
	if err != nil {
		return false, err
	}
	if _, err := io.Copy(checker, r); err != nil {
		return false, err
	}
	return checker.Check(), nil
}

type subChecker struct {
	hash   hash.Hash
	digest []byte
}
type Checker []subChecker

// NewChecker creates a Checker from the given integrity specification, following the format of "integrity metadata" as
// specified by https://www.w3.org/TR/SRI/#the-integrity-attribute
func NewChecker(integrity string) (Checker, error) {
	curPriority := 1 // start at 1 to weed out unrecognized/deprecated algorithms.
	checker := Checker{}
	deprecatedAlgos := []string{}

	for _, expr := range strings.Fields(integrity) {
		matches := exprRegexp.FindStringSubmatch(expr)
		if len(matches) != 4 {
			return nil, fmt.Errorf("%w: couldn't parse hash-with-options: %s", ErrBadIntegrity, expr)
		}
		algo := algos[matches[1]]
		// Record deprecated algorithms. Report an error if all specified algorithms are deprecated.
		if algo.priority == -1 {
			deprecatedAlgos = append(deprecatedAlgos, matches[1])
			continue
		}
		// Note that it is not an error to supply an unrecognized algorithm due to forwards compatibility. In that case,
		// we simply get a priority of 0.
		if algo.priority >= curPriority {
			if algo.priority > curPriority {
				curPriority = algo.priority
				checker = nil
			}
			digest, err := base64.StdEncoding.DecodeString(matches[2])
			if err != nil {
				return nil, fmt.Errorf("%w: couldn't decode base64 payload: %s", ErrBadIntegrity, matches[2])
			}
			checker = append(checker, subChecker{
				hash:   algo.fn(),
				digest: digest,
			})
		}
	}

	if len(checker) == 0 && len(deprecatedAlgos) != 0 {
		return nil, fmt.Errorf("%w: only deprecated hash algorithms found %v", ErrBadIntegrity, deprecatedAlgos)
	}

	return checker, nil
}

// Write adds more data to the underlying running hash(es).
func (c Checker) Write(p []byte) (n int, err error) {
	for _, sub := range c {
		sub.hash.Write(p)
	}
	// Hash writes are guaranteed to not error.
	return len(p), nil
}

// Check returns whether the data written so far (with Write) matches the original integrity metadata.
func (c Checker) Check() bool {
	if len(c) == 0 {
		// If the original integrity was empty, or has no recognized algorithms, the standard says that it should always
		// return true.
		return true
	}
	for _, sub := range c {
		// If any sub-checker passes, it should return true.
		if bytes.Equal(sub.hash.Sum(nil), sub.digest) {
			return true
		}
	}
	return false
}

// Reset resets the Checker to its initial state, so that previously written data no longer counts.
func (c Checker) Reset() {
	for _, sub := range c {
		sub.hash.Reset()
	}
}

// Generate generates a Subresource Integrity metadata from the given algorithm and byte array.
// Unrecognized or deprecated algorithms yield an empty string.
func Generate(algorithm string, bytes []byte) string {
	algo := algos[algorithm]
	if algo.priority <= 0 {
		return ""
	}
	h := algo.fn()
	h.Write(bytes)
	return algorithm + "-" + base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// MustGenerate behaves like Generate, except that an unrecognized or deprecated algortihm causes a panic.
func MustGenerate(algorithm string, bytes []byte) string {
	s := Generate(algorithm, bytes)
	if s == "" {
		panic("unrecognized or deprecated algorithm: " + algorithm)
	}
	return s
}
