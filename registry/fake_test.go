package registry

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFake_GetModuleBazel(t *testing.T) {
	fake := NewFake("fake")
	fake.AddModule(t, "A", "1.0", "foo", nil)
	fake.AddModule(t, "A", "2.0", "bar", nil)
	fake.AddModule(t, "B", "1.0", "baz", nil)

	bytes, err := fake.GetModuleBazel("A", "1.0")
	if assert.NoError(t, err) {
		assert.Equal(t, []byte("foo"), bytes)
	}

	bytes, err = fake.GetModuleBazel("A", "2.0")
	if assert.NoError(t, err) {
		assert.Equal(t, []byte("bar"), bytes)
	}

	bytes, err = fake.GetModuleBazel("B", "1.0")
	if assert.NoError(t, err) {
		assert.Equal(t, []byte("baz"), bytes)
	}

	bytes, err = fake.GetModuleBazel("B", "2.0")
	if err == nil {
		t.Errorf("unexpected success getting B@2.0: got %v", string(bytes))
	} else {
		assert.True(t, errors.Is(err, ErrNotFound))
	}
}
