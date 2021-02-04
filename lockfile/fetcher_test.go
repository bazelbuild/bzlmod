package lockfile

import (
	"encoding/json"
	"fmt"
	"github.com/bazelbuild/bzlmod/fetch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWrapper_JSONRoundtrip(t *testing.T) {
	testCases := []fetch.Fetcher{
		&fetch.Archive{
			URLs:        []string{"https://bazel.build/", "https://build.bazel/"},
			Integrity:   "sha256-blah",
			StripPrefix: "",
			Patches:     nil,
		},
		&fetch.Git{
			Repo:    "https://github.com/bazelbuild/bzlmod",
			Commit:  "123456abcdef",
			Patches: []fetch.Patch{{"file1", 1}, {"file2", 0}},
		},
		&fetch.LocalPath{"heh"},
		// TODO: add a module rule one
	}

	for i, fetcher := range testCases {
		msg := fmt.Sprintf("test case #%v", i)
		bytes, err := json.Marshal(WrapFetcher(fetcher))
		require.NoError(t, err, msg)
		wrapper := FetcherWrapper{}
		require.NoError(t, json.Unmarshal(bytes, &wrapper), msg)
		assert.Equal(t, fetcher, wrapper.Unwrap(), msg)
	}
}
