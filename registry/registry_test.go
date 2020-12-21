package registry

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetModuleBazel_NoOverride(t *testing.T) {
	fake1 := NewFake("1")
	fake2 := NewFake("2")

	fake1.AddModuleBazel(t, "A", "1.0", "Afrom1")
	fake2.AddModuleBazel(t, "A", "1.0", "Afrom2")
	fake2.AddModuleBazel(t, "B", "1.0", "Bfrom2")

	registries := []string{fake1.URL(), fake2.URL()}

	bytes, reg, err := GetModuleBazel("A", "1.0", registries, "")
	if assert.NoError(t, err) {
		assert.Equal(t, []byte("Afrom1"), bytes)
		assert.Equal(t, fake1, reg)
	}

	bytes, reg, err = GetModuleBazel("B", "1.0", registries, "")
	if assert.NoError(t, err) {
		assert.Equal(t, []byte("Bfrom2"), bytes)
		assert.Equal(t, fake2, reg)
	}

	bytes, reg, err = GetModuleBazel("C", "1.0", registries, "")
	if err == nil {
		t.Errorf("unexpected success getting C@1.0: got %v", string(bytes))
	} else {
		assert.True(t, errors.Is(err, ErrNotFound))
	}
}

func TestGetModuleBazel_WithOverride(t *testing.T) {
	fake1 := NewFake("1")
	fake2 := NewFake("2")
	fake3 := NewFake("3")

	fake1.AddModuleBazel(t, "A", "1.0", "Afrom1")
	fake2.AddModuleBazel(t, "A", "1.0", "Afrom2")
	fake2.AddModuleBazel(t, "B", "1.0", "Bfrom2")
	fake3.AddModuleBazel(t, "A", "1.0", "Afrom3")

	registries := []string{fake1.URL(), fake2.URL()}

	bytes, reg, err := GetModuleBazel("A", "1.0", registries, fake3.URL())
	if assert.NoError(t, err) {
		assert.Equal(t, []byte("Afrom3"), bytes)
		assert.Equal(t, fake3, reg)
	}

	bytes, reg, err = GetModuleBazel("B", "1.0", registries, fake3.URL())
	if err == nil {
		t.Errorf("unexpected success getting B@1.0: got %v", string(bytes))
	} else {
		assert.True(t, errors.Is(err, ErrNotFound))
	}

	bytes, reg, err = GetModuleBazel("C", "1.0", registries, fake3.URL())
	if err == nil {
		t.Errorf("unexpected success getting C@1.0: got %v", string(bytes))
	} else {
		assert.True(t, errors.Is(err, ErrNotFound))
	}
}
