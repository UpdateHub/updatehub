/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package installifdifferent

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/filemock"
	"github.com/UpdateHub/updatehub/testsmocks/filesystemmock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

const (
	ObjectWithInstallIfDifferentSha256Sum = `{
        "mode": "test",
        "target": "/tmp/dev/xx1",
        "target-type": "device",
        "target-path": "/path",
        "install-if-different": "sha256sum",
        "sha256sum": "b5a2c96250612366ea272ffac6d9744aaf4b45aacd96aa7cfcb931ee3b558259"
	}`

	ObjectWithInstallIfDifferentPattern = `{
        "mode": "test",
        "target": "/tmp/dev/xx1",
        "target-type": "device",
        "target-path": "/path",
        "install-if-different": {
            "version": "2.0",
            "pattern": {
                "regexp": "\\d\\.\\d",
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

	ObjectWithInstallIfDifferentUnknownStringFormat = `{
        "mode": "test",
        "target": "/tmp/dev/xx1",
        "target-type": "device",
        "target-path": "/path",
        "install-if-different": "foo"
	}`

	ObjectWithInstallIfDifferentPatternWithArrayPattern = `{
        "mode": "test",
        "target": "/tmp/dev/xx1",
        "target-type": "device",
        "target-path": "/path",
        "install-if-different": {
            "version": "2.0",
            "pattern": [],
            "extra": "property"
        }
	}`

	ObjectWithInstallIfDifferentPatternWithInvalidPattern = `{
        "mode": "test",
        "target": "/tmp/dev/xx1",
        "target-type": "device",
        "target-path": "/path",
        "install-if-different": {
            "version": "2.0",
            "pattern": {
                "regexp": ".+",
                "seek": -1,
                "buffer-size": -1
            },
            "extra": "property"
        }
	}`

	testObjectGetTargetReturn = "/tmp/get-target-return"
)

type testObject struct {
	metadata.ObjectMetadata
	TargetProvider
}

func (to *testObject) GetTarget() string {
	return testObjectGetTargetReturn
}

func (to *testObject) SetupTarget(target afero.File) {
}

type testObjectWithoutIIDSupport struct {
	metadata.ObjectMetadata
}

func TestProceedWithoutInstallIfDifferentOnObject(t *testing.T) {
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

	o, err := metadata.NewObjectMetadata([]byte(ObjectWithoutInstallIfDifferent))
	assert.NoError(t, err)

	install, err := iif.Proceed(o)
	assert.NoError(t, err)
	assert.True(t, install)
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

	o, err := metadata.NewObjectMetadata([]byte(ObjectWithInstallIfDifferentSha256Sum))
	assert.NoError(t, err)

	install, err := iif.Proceed(o)
	assert.NoError(t, err)
	assert.True(t, install)
}

func TestProceedWithSha256SumMatch(t *testing.T) {
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
	assert.NoError(t, err)
	assert.False(t, install)
}

func TestProceedWithSha256SumWithoutMatch(t *testing.T) {
	memFs := afero.NewMemMapFs()

	err := afero.WriteFile(memFs, testObjectGetTargetReturn, []byte("no-match"), 0666)
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
	assert.NoError(t, err)
	assert.True(t, install)
}

func TestProceedWithSha256SumWithOpenError(t *testing.T) {
	memFs := afero.NewMemMapFs()

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
	assert.EqualError(t, err, fmt.Sprintf("open %s: file does not exist", testObjectGetTargetReturn))
	assert.False(t, install)
}

func TestProceedWithPatternWithVersionMatch(t *testing.T) {
	// when version match means it should NOT install
	fs := afero.NewOsFs()

	var content bytes.Buffer
	for i := 0; i < 1024; i++ {
		content.WriteString(" ")
	}
	content.WriteString("2.0   ")

	err := afero.WriteFile(fs, testObjectGetTargetReturn, content.Bytes(), 0666)
	assert.NoError(t, err)
	defer fs.Remove(testObjectGetTargetReturn)

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return &testObject{} },
	})
	defer mode.Unregister()

	iif := &DefaultImpl{fs}

	o, err := metadata.NewObjectMetadata([]byte(ObjectWithInstallIfDifferentPattern))
	assert.NoError(t, err)

	install, err := iif.Proceed(o)
	assert.NoError(t, err)
	assert.False(t, install)
}

func TestProceedWithPatternWithoutVersionMatch(t *testing.T) {
	// when version don't match means it should install
	fs := afero.NewOsFs()

	var content bytes.Buffer
	for i := 0; i < 1024; i++ {
		content.WriteString(" ")
	}
	content.WriteString("3.3   ")

	// the string "dummy" won't match the pattern in "ObjectWithInstallIfDifferentPattern"
	err := afero.WriteFile(fs, testObjectGetTargetReturn, content.Bytes(), 0666)
	assert.NoError(t, err)
	defer fs.Remove(testObjectGetTargetReturn)

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return &testObject{} },
	})
	defer mode.Unregister()

	iif := &DefaultImpl{fs}

	o, err := metadata.NewObjectMetadata([]byte(ObjectWithInstallIfDifferentPattern))
	assert.NoError(t, err)

	install, err := iif.Proceed(o)
	assert.NoError(t, err)
	assert.True(t, install)
}

func TestProceedWithPatternWithUnknownPattern(t *testing.T) {
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

	o, err := metadata.NewObjectMetadata([]byte(ObjectWithInstallIfDifferentPatternWithArrayPattern))
	assert.NoError(t, err)

	install, err := iif.Proceed(o)
	assert.EqualError(t, err, "failed to parse install-if-different object: install-if-different pattern is unknown")
	assert.False(t, install)
}

func TestProceedWithPatternWithInvalidPattern(t *testing.T) {
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

	o, err := metadata.NewObjectMetadata([]byte(ObjectWithInstallIfDifferentPatternWithInvalidPattern))
	assert.NoError(t, err)

	install, err := iif.Proceed(o)
	assert.NoError(t, err)
	assert.False(t, install)
}

func TestProceedWithPatternWithCaptureError(t *testing.T) {
	fs := &filesystemmock.FileSystemBackendMock{}
	fs.On("OpenFile", testObjectGetTargetReturn, os.O_RDONLY, os.FileMode(0)).Return((*filemock.FileMock)(nil), fmt.Errorf("open error"))

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return &testObject{} },
	})
	defer mode.Unregister()

	iif := &DefaultImpl{fs}

	o, err := metadata.NewObjectMetadata([]byte(ObjectWithInstallIfDifferentPattern))
	assert.NoError(t, err)

	install, err := iif.Proceed(o)
	assert.EqualError(t, err, "open error")
	assert.False(t, install)

	fs.AssertExpectations(t)
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

func TestProceedWithUnknownStringFormatError(t *testing.T) {
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

	o, err := metadata.NewObjectMetadata([]byte(ObjectWithInstallIfDifferentUnknownStringFormat))
	assert.NoError(t, err)

	install, err := iif.Proceed(o)
	assert.EqualError(t, err, "unknown install-if-different format")
	assert.False(t, install)
}
