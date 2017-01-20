package metadata

import (
	"testing"

	"bitbucket.org/ossystems/agent/pkg"
	"bitbucket.org/ossystems/agent/plugins"
	"github.com/stretchr/testify/assert"
)

type TestObject struct {
	pkg.Object
}

func TestObjectFROMJson(t *testing.T) {
	plugins.RegisterPlugin("test", plugins.Plugin{
		Mode:              "test",
		CheckRequirements: func() error { return nil },
		Instantiate:       func() interface{} { return TestObject{} },
	})

	obj, err := PackageObjectFromJSON([]byte("{ \"mode\": \"test\" }"))
	if !assert.NotNil(t, obj) {
		t.Fatal(err)
	}

	assert.IsType(t, TestObject{}, obj)
}

func TestObjectFromInvalidJson(t *testing.T) {
	obj, err := PackageObjectFromJSON([]byte("invalid"))
	assert.Nil(t, obj)
	assert.Error(t, err)
}
