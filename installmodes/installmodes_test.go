package installmodes

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestObject struct {
}

func TestRegisterInstallMode(t *testing.T) {
	mode := RegisterInstallMode(InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return &TestObject{} },
	})

	defer mode.Unregister()

	obj, err := GetObject("test")
	if !assert.NotNil(t, obj) {
		t.Fatal(err)
	}
}

func TestUnregisterInstallMode(t *testing.T) {
	mode := RegisterInstallMode(InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return &TestObject{} },
	})

	defer mode.Unregister()

	obj, err := GetObject("test")
	if !assert.NotNil(t, obj) {
		t.Fatal(err)
	}

	mode.Unregister()

	obj, err = GetObject("test")
	if !assert.Nil(t, obj) {
		t.Fatal(err)
	}
}

func TestGetObjectNotFound(t *testing.T) {
	_, err := GetObject("test")

	if assert.Error(t, err) {
		assert.Equal(t, errors.New("Object not found"), err)
	}
}

func TestCheckRequirements(t *testing.T) {
	mode := RegisterInstallMode(InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
	})

	defer mode.Unregister()

	err := CheckRequirements()
	assert.NoError(t, err)
}

func TestFailCheckRequirements(t *testing.T) {
	mode := RegisterInstallMode(InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return errors.New("") },
	})

	defer mode.Unregister()

	err := CheckRequirements()
	assert.Error(t, err)
}
