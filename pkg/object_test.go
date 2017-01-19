package pkg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObjectCheckrequirementsNil(t *testing.T) {
	o := ObjectData{}
	assert.Nil(t, o.CheckRequirements())
}

func TestObjectSetupNil(t *testing.T) {
	o := ObjectData{}
	assert.Nil(t, o.Setup())
}

func TestObjectInstallNil(t *testing.T) {
	o := ObjectData{}
	assert.Nil(t, o.Install())
}

func TestObjectCleanupNil(t *testing.T) {
	o := ObjectData{}
	assert.Nil(t, o.Cleanup())
}

func TestObjectFROMJson(t *testing.T) {
	/*	testCases := []struct {
			Name string
			Mode string
			Type interface{}
		}{
			{
				"Copy",
				"copy",
				&Copy{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				obj, err := ObjectFromJSON([]byte(fmt.Sprintf("{ \"mode\": \"%s\" }", tc.Mode)))
				if !assert.NotNil(t, obj) {
					t.Fatal(err)
				}

				assert.IsType(t, tc.Type, obj)
			})
		}*/
}

func TestObjectFromInvalidJson(t *testing.T) {
	obj, err := ObjectFromJSON([]byte("invalid"))
	assert.Nil(t, obj)
	assert.Error(t, err)
}
