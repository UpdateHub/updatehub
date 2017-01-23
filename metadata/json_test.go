package metadata

import (
	"errors"
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

	validJSONMetadataWithCompressedObject = `{
	  "product-uid": "0123456789",
	  "objects": [
	    [
	      { "mode": "compressed-object", "compressed": true }
	    ]
	  ]
	}`

	validJSONMetadataWithoutCompressedObject = `{
	  "product-uid": "0123456789",
	  "objects": [
	    [
	      { "mode": "test", "compressed": true }
	    ]
	  ]
	}`
)

type TestObject struct {
	Object
}

type TestObjectCompressed struct {
	Object
	CompressedObject
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

func TestCompressedObject(t *testing.T) {
	installmodes.RegisterInstallMode("compressed-object", installmodes.InstallMode{
		Mode:              "compressed-object",
		CheckRequirements: func() error { return nil },
		Instantiate:       func() interface{} { return TestObjectCompressed{} },
	})

	obj, err := FromJSON([]byte(validJSONMetadataWithCompressedObject))
	if !assert.NotNil(t, obj) {
		t.Fatal(err)
	}
}

func TestInvalidCompressedObject(t *testing.T) {
	installmodes.RegisterInstallMode("test", installmodes.InstallMode{
		Mode:              "test",
		CheckRequirements: func() error { return nil },
		Instantiate:       func() interface{} { return TestObject{} },
	})

	_, err := FromJSON([]byte(validJSONMetadataWithoutCompressedObject))
	if assert.Error(t, err) {
		assert.Equal(t, err, errors.New("Compressed object does not embed CompressedObject struct"))
	}
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
