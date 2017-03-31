/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package metadata

import (
	"errors"
	"testing"

	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/stretchr/testify/assert"
)

type TestObjectCompressed struct {
	Object
	CompressedObject
}

func TestMetadataFromValidJson(t *testing.T) {
	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return TestObject{} },
	})

	defer mode.Unregister()

	m, err := NewUpdateMetadata([]byte(ValidJSONMetadata))
	if !assert.NotNil(t, m) {
		t.Fatal(err)
	}

	assert.NotEmpty(t, m.Objects)
	assert.NotEmpty(t, m.Objects[0])
	assert.IsType(t, TestObject{}, m.Objects[0][0])
}

func TestMetadataFromValidJsonWithActiveInactive(t *testing.T) {
	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return TestObject{} },
	})

	defer mode.Unregister()

	m, err := NewUpdateMetadata([]byte(ValidJSONMetadataWithActiveInactive))
	if !assert.NotNil(t, m) {
		t.Fatal(err)
	}

	assert.NotEmpty(t, m.Objects)
	assert.Equal(t, 2, len(m.Objects))
	assert.NotEmpty(t, m.Objects[0])
	assert.NotEmpty(t, m.Objects[1])
	assert.IsType(t, TestObject{}, m.Objects[0][0])
	assert.IsType(t, TestObject{}, m.Objects[1][0])
}

func TestCompressedObject(t *testing.T) {
	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "compressed-object",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return TestObjectCompressed{} },
	})

	defer mode.Unregister()

	obj, err := NewUpdateMetadata([]byte(ValidJSONMetadataWithCompressedObject))
	if !assert.NotNil(t, obj) {
		t.Fatal(err)
	}
}

func TestInvalidCompressedObject(t *testing.T) {
	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return TestObject{} },
	})

	defer mode.Unregister()

	_, err := NewUpdateMetadata([]byte(ValidJSONMetadataWithoutCompressedObject))
	if assert.Error(t, err) {
		assert.Equal(t, err, errors.New("Compressed object does not embed CompressedObject struct"))
	}
}
