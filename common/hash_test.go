package common

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestHash_SliceIdentity(t *testing.T) {
	assert.Equal(t,
		Hash("abc", []string{"def", "ghi"}, "jkl"),
		Hash("abc", []string{"def", "ghi"}, "jkl"),
	)
}
