package metadata

import (
	"testing"

	"bitbucket.org/ossystems/agent/installmodes"
	"github.com/stretchr/testify/assert"
)

const (
	validJSONMetadata = `{
	  "product-uid": "0123456789",
	  "objects": [
	    [
	      { "mode": "test" }
	    ]
	  ]
	}`
)

type TestObject struct {
	Object
}

func TestMetadataFromValidJson(t *testing.T) {
	installmodes.RegisterInstallMode("test", installmodes.InstallMode{
		Mode:              "test",
		CheckRequirements: func() error { return nil },
		Instantiate:       func() interface{} { return TestObject{} },
	})

	m, err := FromJSON([]byte(validJSONMetadata))
	if !assert.NotNil(t, m) {
		t.Fatal(err)
	}

	assert.NotEmpty(t, m.Objects)
	assert.NotEmpty(t, m.Objects[0])
	assert.IsType(t, TestObject{}, m.Objects[0][0])
}

func TestObjectFromValidJson(t *testing.T) {
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
