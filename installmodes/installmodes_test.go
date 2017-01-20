package installmodes

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestObject struct {
}

func TestRegisterInstallMode(t *testing.T) {
	RegisterInstallMode("test1", InstallMode{
		CheckRequirements: func() error { return nil },
		Instantiate:       func() interface{} { return &TestObject{} },
	})

	obj, err := GetObject("test1")
	if !assert.NotNil(t, obj) {
		t.Fatal(err)
	}
}

func TestGetObjectNotFound(t *testing.T) {
	_, err := GetObject("")

	if assert.Error(t, err) {
		assert.Equal(t, errors.New("Object not found"), err)
	}
}

func TestCheckRequirements(t *testing.T) {
	RegisterInstallMode("test2", InstallMode{
		CheckRequirements: func() error { return nil },
	})

	err := CheckRequirements()
	assert.NoError(t, err)
}

func TestFailCheckRequirements(t *testing.T) {
	RegisterInstallMode("test3", InstallMode{
		CheckRequirements: func() error { return errors.New("") },
	})

	err := CheckRequirements()
	assert.Error(t, err)
}
