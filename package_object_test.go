package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPackageObjectSetupNil(t *testing.T) {
	o := PackageObjectData{}
	assert.Nil(t, o.Setup())
}

func TestPackageObjectCheckrequirementsNil(t *testing.T) {
	o := PackageObjectData{}
	assert.Nil(t, o.CheckRequirements())
}

func TestPackageObjectFROMJson(t *testing.T) {
	testCases := []struct {
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
			obj, err := PackageObjectFromJSON([]byte(fmt.Sprintf("{ \"mode\": \"%s\" }", tc.Mode)))
			if !assert.NotNil(t, obj) {
				t.Fatal(err)
			}

			assert.IsType(t, tc.Type, obj)
		})
	}
}

func TestPackageObjectFromInvalidJson(t *testing.T) {
	obj, err := PackageObjectFromJSON([]byte("invalid"))
	assert.Nil(t, obj)
	assert.Error(t, err)
}
