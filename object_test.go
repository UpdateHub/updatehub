package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObjectSetupNil(t *testing.T) {
	o := Object{}
	assert.Nil(t, o.Setup())
}

func TestObjectCheckrequirementsNil(t *testing.T) {
	o := Object{}
	assert.Nil(t, o.CheckRequirements())
}
