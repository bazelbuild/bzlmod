package registry

import (
	"errors"
	"github.com/bazelbuild/bzlmod/fetch"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFake(t *testing.T) {
	fake := NewFake("fake")
	fake.AddModule(t, "A", "1.0", "foo", &fetch.LocalPath{"A/1.0"})
	fake.AddModule(t, "A", "2.0", "bar", &fetch.LocalPath{"A/2.0"})
	fake.AddModule(t, "B", "1.0", "baz", &fetch.LocalPath{"B/1.0"})

	bytes, err := fake.GetModuleBazel("A", "1.0")
	if assert.NoError(t, err) {
		assert.Equal(t, []byte("foo"), bytes)
	}
	fetcher, err := fake.GetFetcher("A", "1.0")
	if assert.NoError(t, err) {
		assert.Equal(t, &fetch.LocalPath{"A/1.0"}, fetcher)
	}

	bytes, err = fake.GetModuleBazel("A", "2.0")
	if assert.NoError(t, err) {
		assert.Equal(t, []byte("bar"), bytes)
	}
	fetcher, err = fake.GetFetcher("A", "2.0")
	if assert.NoError(t, err) {
		assert.Equal(t, &fetch.LocalPath{"A/2.0"}, fetcher)
	}

	bytes, err = fake.GetModuleBazel("B", "1.0")
	if assert.NoError(t, err) {
		assert.Equal(t, []byte("baz"), bytes)
	}
	fetcher, err = fake.GetFetcher("B", "1.0")
	if assert.NoError(t, err) {
		assert.Equal(t, &fetch.LocalPath{"B/1.0"}, fetcher)
	}

	bytes, err = fake.GetModuleBazel("B", "2.0")
	if err == nil {
		t.Errorf("unexpected success getting B@2.0: got %v", string(bytes))
	} else {
		assert.True(t, errors.Is(err, ErrNotFound))
	}
}
