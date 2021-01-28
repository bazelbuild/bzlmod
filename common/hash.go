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
		if bs, ok := i.([]byte); ok {
			_, _ = hash.Write(bs)
			_, _ = fmt.Fprint(hash, '$')
		} else {
			_, _ = fmt.Fprint(hash, i, '$')
		}
	}
	return base32.StdEncoding.EncodeToString(hash.Sum(nil))
}
