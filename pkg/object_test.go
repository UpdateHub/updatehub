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
