/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package installifdifferent

import (
	"fmt"
	"testing"

	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

const (
	ObjectWithInstallIfDifferentSha256Sum = `{
        "mode": "test",
        "target": "/tmp/dev/xx1",
        "target-type": "device",
        "target-path": "/path",
        "install-if-different": "73a625b0271d75702cc4205061da9efc6112e9fe94033ed0f9033157109ebe75"
	}`

	ObjectWithInstallIfDifferentPattern = `{
        "mode": "test",
        "target": "/tmp/dev/xx1",
        "target-type": "device",
        "target-path": "/path",
        "install-if-different": {
            "version": "2.0",
            "pattern": {
                "regexp": ".+",
                "seek": 1024,
                "buffer-size": 2024
            },
            "extra": "property"
        }
	}`

	ObjectWithoutInstallIfDifferent = `{
        "mode": "test",
        "target": "/tmp/dev/xx1",
        "target-type": "device",
        "target-path": "/path"
	}`

	ObjectWithInstallIfDifferentUnknownFormat = `{
        "mode": "test",
        "target": "/tmp/dev/xx1",
        "target-type": "device",
        "target-path": "/path",
        "install-if-different": [ 1, 2, 3 ]
	}`

	testObjectGetTargetReturn = "/tmp/get-target-return"
)

type testObject struct {
	metadata.ObjectMetadata
	TargetGetter
}

func (to *testObject) GetTarget() string {
	return testObjectGetTargetReturn
}

type testObjectWithoutIIDSupport struct {
	metadata.ObjectMetadata
}

func TestProceedWithoutInstallIfDifferentSupport(t *testing.T) {
	memFs := afero.NewMemMapFs()

	err := afero.WriteFile(memFs, testObjectGetTargetReturn, []byte("dummy"), 0666)
	assert.NoError(t, err)
	defer memFs.Remove(testObjectGetTargetReturn)

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return &testObjectWithoutIIDSupport{} },
	})
	defer mode.Unregister()

	iif := &DefaultImpl{memFs}

	o, err := metadata.NewObjectMetadata([]byte(ObjectWithoutInstallIfDifferent))
	assert.NoError(t, err)

	install, err := iif.Proceed(o)
	assert.NoError(t, err)
	assert.True(t, install)
}

func TestProceedWithSha256Sum(t *testing.T) {
	memFs := afero.NewMemMapFs()

	err := afero.WriteFile(memFs, testObjectGetTargetReturn, []byte("dummy"), 0666)
	assert.NoError(t, err)
	defer memFs.Remove(testObjectGetTargetReturn)

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return &testObject{} },
	})
	defer mode.Unregister()

	iif := &DefaultImpl{memFs}

	o, err := metadata.NewObjectMetadata([]byte(ObjectWithInstallIfDifferentSha256Sum))
	assert.NoError(t, err)

	install, err := iif.Proceed(o)
	/*
		    FIXME: this is supposed to be successful like this commented
		    asserts. We are testing for "non-implemented yet" temporarily as
		    an itermediate step since this feature is enormous.

			assert.NoError(t, err)
			assert.True(t, install)
	*/

	assert.EqualError(t, err, "installIfDifferent: Sha256Sum not yet implemented")
	assert.False(t, install)
}

func TestProceedWithPattern(t *testing.T) {
	memFs := afero.NewMemMapFs()

	err := afero.WriteFile(memFs, testObjectGetTargetReturn, []byte("dummy"), 0666)
	assert.NoError(t, err)
	defer memFs.Remove(testObjectGetTargetReturn)

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return &testObject{} },
	})
	defer mode.Unregister()

	iif := &DefaultImpl{memFs}

	o, err := metadata.NewObjectMetadata([]byte(ObjectWithInstallIfDifferentPattern))
	assert.NoError(t, err)

	install, err := iif.Proceed(o)
	/*
		    FIXME: this is supposed to be successful like this commented
		    asserts. We are testing for "non-implemented yet" temporarily as
		    an itermediate step since this feature is enormous.

			assert.NoError(t, err)
			assert.True(t, install)
	*/

	assert.EqualError(t, err, "installIfDifferent: Pattern not yet implemented")
	assert.False(t, install)
}

func TestProceedWithUnknownFormatError(t *testing.T) {
	memFs := afero.NewMemMapFs()

	err := afero.WriteFile(memFs, testObjectGetTargetReturn, []byte("dummy"), 0666)
	assert.NoError(t, err)
	defer memFs.Remove(testObjectGetTargetReturn)

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return &testObject{} },
	})
	defer mode.Unregister()

	iif := &DefaultImpl{memFs}

	o, err := metadata.NewObjectMetadata([]byte(ObjectWithInstallIfDifferentUnknownFormat))
	assert.NoError(t, err)

	install, err := iif.Proceed(o)
	assert.EqualError(t, err, "unknown install-if-different format")
	assert.False(t, install)
}

func TestProceedWithTargetNotFoundError(t *testing.T) {
	memFs := afero.NewMemMapFs()

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return &testObject{} },
	})
	defer mode.Unregister()

	iif := &DefaultImpl{memFs}

	o, err := metadata.NewObjectMetadata([]byte(ObjectWithInstallIfDifferentUnknownFormat))
	assert.NoError(t, err)

	install, err := iif.Proceed(o)
	assert.EqualError(t, err, fmt.Sprintf("install-if-different: target '%s' not found", testObjectGetTargetReturn))
	assert.False(t, install)
}
