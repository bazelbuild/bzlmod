package fetch

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWrapper_JSONRoundtrip(t *testing.T) {
	testCases := []Fetcher{
		&Archive{
			URLs:        []string{"https://bazel.build/", "https://build.bazel/"},
			Integrity:   "sha256-blah",
			StripPrefix: "",
			PatchFiles:  nil,
		},
		&Git{
			Repo:       "https://github.com/bazelbuild/bzlmod",
			Commit:     "123456abcdef",
			PatchFiles: []string{"just kidding"},
		},
		&LocalPath{"heh"},
	}

	for i, fetcher := range testCases {
		msg := fmt.Sprintf("test case #%v", i)
		bytes, err := json.Marshal(Wrap(fetcher))
		require.NoError(t, err, msg)
		wrapper := Wrapper{}
		require.NoError(t, json.Unmarshal(bytes, &wrapper), msg)
		assert.Equal(t, fetcher, wrapper.Unwrap(), msg)
	}
}
