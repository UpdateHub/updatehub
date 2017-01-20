package metadata

import (
	"testing"

	"bitbucket.org/ossystems/agent/installmodes"
	"github.com/stretchr/testify/assert"
)

type TestObject struct {
	Object
}

func TestObjectFROMJson(t *testing.T) {
	installmodes.RegisterInstallMode("test", installmodes.InstallMode{
		Mode:              "test",
		CheckRequirements: func() error { return nil },
		Instantiate:       func() interface{} { return TestObject{} },
	})

	obj, err := ObjectFromJSON([]byte("{ \"mode\": \"test\" }"))
	if !assert.NotNil(t, obj) {
		t.Fatal(err)
	}

	assert.IsType(t, TestObject{}, obj)
}

func TestObjectFromInvalidJson(t *testing.T) {
	obj, err := ObjectFromJSON([]byte("invalid"))
	assert.Nil(t, obj)
	assert.Error(t, err)
}
