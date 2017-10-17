/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package metadata

import (
	"testing"

	"github.com/updatehub/updatehub/installmodes"
	"github.com/stretchr/testify/assert"
)

type TestObject struct {
	Object
}

func TestObjectFromValidJson(t *testing.T) {
	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return TestObject{} },
	})

	defer mode.Unregister()

	obj, err := NewObjectMetadata([]byte("{ \"mode\": \"test\" }"))
	if !assert.NotNil(t, obj) {
		t.Fatal(err)
	}

	assert.IsType(t, TestObject{}, obj)
}

func TestObjectFromInvalidJson(t *testing.T) {
	obj, err := NewObjectMetadata([]byte("invalid"))
	assert.Nil(t, obj)
	assert.Error(t, err)
}
