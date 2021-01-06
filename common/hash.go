package common

import (
	"crypto/sha1"
	"encoding/base32"
	"fmt"
)

// Hash uses SHA1 to combine everything and then uses base32 to encode the result. The result is a string of length 32.
func Hash(s ...interface{}) string {
	hash := sha1.New()
	for _, i := range s {
		_, _ = fmt.Fprintf(hash, "%v$", i)
	}
	return base32.StdEncoding.EncodeToString(hash.Sum(nil))
}
