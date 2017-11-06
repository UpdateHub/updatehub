/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package installifdifferent

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/updatehub/updatehub/testsmocks/filemock"
	"github.com/updatehub/updatehub/testsmocks/filesystemmock"
)

func TestIsValid(t *testing.T) {
	valid1 := &Pattern{
		Type:       UBootPattern,
		RegExp:     `U-Boot (\S+) \(.*\)`,
		Seek:       2,
		BufferSize: 3,
	}
	assert.True(t, valid1.IsValid())

	valid2 := &Pattern{
		Type:       LinuxKernelPattern,
		RegExp:     ``,
		Seek:       0,
		BufferSize: 0,
	}
	assert.True(t, valid2.IsValid())
}

func TestIsValidWithInvalidRegexp(t *testing.T) {
	invalid := &Pattern{
		Type:       UBootPattern,
		RegExp:     `[a-z`,
		Seek:       0,
		BufferSize: 0,
	}
	assert.False(t, invalid.IsValid())
}

func TestIsValidWithInvalidSeek(t *testing.T) {
	invalid := &Pattern{
		Type:       LinuxKernelPattern,
		RegExp:     ``,
		Seek:       -1,
		BufferSize: 0,
	}

	assert.False(t, invalid.IsValid())
}

func TestIsValidWithInvalidBufferSize(t *testing.T) {
	invalid := &Pattern{
		Type:       CustomPattern,
		RegExp:     ``,
		Seek:       0,
		BufferSize: -1,
	}

	assert.False(t, invalid.IsValid())
}

var (
	InstallIfDifferentObjectWithUBootPattern = map[string]interface{}{
		"version": "2.0",
		"pattern": "u-boot",
		"extra":   "property",
	}

	InstallIfDifferentObjectWithLinuxKernelPattern = map[string]interface{}{
		"version": "4.7.4-1-ARCH",
		"pattern": "linux-kernel",
		"extra":   "property",
	}

	InstallIfDifferentObjectWithCustomPattern = map[string]interface{}{
		"version": "2.0",
		"pattern": map[string]interface{}{
			"regexp":      ".+",
			"seek":        1024,
			"buffer-size": 2024,
		},
		"extra": "property",
	}

	InstallIfDifferentObjectWithCustomPatternAndUnmarshalFailure = map[string]interface{}{
		"version": "2.0",
		"pattern": map[string]interface{}{
			"regexp":      ".+",
			"seek":        Pattern{},
			"buffer-size": 2024,
		},
		"extra": "property",
	}
)

func TestNewPatternFromInstallIfDifferentObject(t *testing.T) {
	memFs := afero.NewMemMapFs()

	p, err := NewPatternFromInstallIfDifferentObject(memFs, InstallIfDifferentObjectWithUBootPattern)
	assert.NoError(t, err)
	assert.Equal(t, UBootPattern, p.Type)
	assert.Equal(t, `U-Boot(?: SPL)? (\S+) \(.*\)`, p.RegExp)
	assert.Equal(t, int64(0), p.Seek)
	assert.Equal(t, int64(0), p.BufferSize)
	assert.True(t, p.IsValid())

	p, err = NewPatternFromInstallIfDifferentObject(memFs, InstallIfDifferentObjectWithLinuxKernelPattern)
	assert.NoError(t, err)
	assert.Equal(t, LinuxKernelPattern, p.Type)
	assert.Equal(t, ``, p.RegExp)
	assert.Equal(t, int64(0), p.Seek)
	assert.Equal(t, int64(0), p.BufferSize)
	assert.True(t, p.IsValid())

	p, err = NewPatternFromInstallIfDifferentObject(memFs, InstallIfDifferentObjectWithCustomPattern)
	assert.NoError(t, err)
	assert.Equal(t, CustomPattern, p.Type)
	assert.Equal(t, `.+`, p.RegExp)
	assert.Equal(t, int64(1024), p.Seek)
	assert.Equal(t, int64(2024), p.BufferSize)
	assert.True(t, p.IsValid())

	p, err = NewPatternFromInstallIfDifferentObject(memFs, InstallIfDifferentObjectWithCustomPatternAndUnmarshalFailure)
	assert.IsType(t, &json.UnmarshalTypeError{}, err)
	assert.Nil(t, p)

	p, err = NewPatternFromInstallIfDifferentObject(memFs, map[string]interface{}{})
	assert.EqualError(t, err, "install-if-different pattern is unknown")
	assert.Nil(t, p)
}

func TestCaptureWithUnknownPattern(t *testing.T) {
	fs := afero.NewMemMapFs()

	p := &Pattern{Type: -1, FileSystemBackend: fs}

	assert.False(t, p.IsValid())
	assert.Equal(t, PatternType(-1), p.Type)

	version, err := p.Capture("/dummy-file")
	assert.EqualError(t, err, "unknown pattern type")
	assert.Equal(t, "", version)
}

func TestCaptureWithLinuxKernelPatternError(t *testing.T) {
	targetFile := "/dummy-file"
	fsm := &filesystemmock.FileSystemBackendMock{}
	fsm.On("Open", targetFile).Return((*filemock.FileMock)(nil), fmt.Errorf("open error"))

	p := &Pattern{
		Type:              LinuxKernelPattern,
		FileSystemBackend: fsm,
	}

	assert.True(t, p.IsValid())
	assert.Equal(t, LinuxKernelPattern, p.Type)

	version, err := p.Capture(targetFile)
	assert.EqualError(t, err, "open error")
	assert.Equal(t, "", version)

	fsm.AssertExpectations(t)
}

func TestCaptureWithCustomPatternError(t *testing.T) {
	targetFile := "/dummy-file"
	fsm := &filesystemmock.FileSystemBackendMock{}
	fsm.On("Open", targetFile).Return((*filemock.FileMock)(nil), fmt.Errorf("open error"))

	p := &Pattern{
		Type:              CustomPattern,
		FileSystemBackend: fsm,
	}

	assert.True(t, p.IsValid())
	assert.Equal(t, CustomPattern, p.Type)

	version, err := p.Capture(targetFile)
	assert.EqualError(t, err, "open error")
	assert.Equal(t, "", version)

	fsm.AssertExpectations(t)
}

func TestCaptureWithCustomPattern(t *testing.T) {
	fs := afero.NewOsFs()

	tempDirPath, err := afero.TempDir(fs, "", "pattern-test")
	assert.NoError(t, err)
	testFile := path.Join(tempDirPath, "test-file")
	defer fs.RemoveAll(tempDirPath)

	decoded, _ := hex.DecodeString("5f5f5f312e305f5f5f") // ___1.0___

	_ = afero.WriteFile(fs, testFile, decoded, 0666)

	expectedVersion := "1.0"
	expectedRegexp := "\\d\\.\\d"
	expectedSeek := int64(3)
	expectedBufferSize := int64(5)

	pattern := map[string]interface{}{
		"pattern": map[string]interface{}{
			"regexp":      expectedRegexp,
			"seek":        expectedSeek,
			"buffer-size": expectedBufferSize,
		},
	}

	p, err := NewPatternFromInstallIfDifferentObject(fs, pattern)
	assert.NoError(t, err)

	assert.True(t, p.IsValid())
	assert.Equal(t, CustomPattern, p.Type)
	assert.Equal(t, expectedRegexp, p.RegExp)
	assert.Equal(t, expectedBufferSize, p.BufferSize)

	version, err := p.Capture(testFile)
	assert.NoError(t, err)
	assert.Equal(t, expectedVersion, version)
}

func TestCaptureWithLinuxKernelPattern(t *testing.T) {
	fs := afero.NewOsFs()

	tempDirPath, err := afero.TempDir(fs, "", "pattern-test")
	assert.NoError(t, err)
	testFile := path.Join(tempDirPath, "test-file")
	defer fs.RemoveAll(tempDirPath)

	file, _ := fs.Create(testFile)

	file.Seek(510, io.SeekStart)

	magic := uint16(0xaa55)
	binary.Write(file, binary.LittleEndian, magic) // 2 bytes

	file.Seek(526, io.SeekStart)

	expectedVersion := "13.08.88"

	versionOffset := uint16(0x36e0)
	versionLength := 128

	binary.Write(file, binary.LittleEndian, versionOffset) // 2 bytes

	file.Seek(int64(versionOffset)+0x200, io.SeekStart)
	versionData := make([]byte, versionLength)
	copy(versionData, []byte(expectedVersion))
	file.Write(versionData)

	file.Close()

	pattern := map[string]interface{}{
		"pattern": "linux-kernel",
	}

	p, err := NewPatternFromInstallIfDifferentObject(fs, pattern)
	assert.NoError(t, err)

	assert.True(t, p.IsValid())
	assert.Equal(t, LinuxKernelPattern, p.Type)

	version, err := p.Capture(testFile)
	assert.NoError(t, err)
	assert.Equal(t, expectedVersion, version)
}

func TestCaptureWithUbootPattern(t *testing.T) {
	fs := afero.NewOsFs()

	tempDirPath, err := afero.TempDir(fs, "", "pattern-test")
	assert.NoError(t, err)
	testFile := path.Join(tempDirPath, "test-file")
	defer fs.RemoveAll(tempDirPath)

	//                              U-Boot 13.08.1988 (13/08/1988)
	decoded, _ := hex.DecodeString("01552d426f6f742031332e30382e31393838202831332f30382f313938382902")

	_ = afero.WriteFile(fs, testFile, decoded, 0666)

	expectedVersion := "13.08.1988"

	pattern := map[string]interface{}{
		"pattern": "u-boot",
	}

	p, err := NewPatternFromInstallIfDifferentObject(fs, pattern)
	assert.NoError(t, err)

	assert.True(t, p.IsValid())
	assert.Equal(t, UBootPattern, p.Type)

	version, err := p.Capture(testFile)
	assert.NoError(t, err)
	assert.Equal(t, expectedVersion, version)
}

func TestCaptureWithUbootSplPattern(t *testing.T) {
	fs := afero.NewOsFs()

	tempDirPath, err := afero.TempDir(fs, "", "pattern-test")
	assert.NoError(t, err)
	testFile := path.Join(tempDirPath, "test-file")
	defer fs.RemoveAll(tempDirPath)

	//                              U-Boot SPL 13.08.1988 (13/08/1988)
	decoded, _ := hex.DecodeString("01552d426f6f742053504c2031332e30382e31393838202831332f30382f313938382902")

	_ = afero.WriteFile(fs, testFile, decoded, 0666)

	expectedVersion := "13.08.1988"

	pattern := map[string]interface{}{
		"pattern": "u-boot",
	}

	p, err := NewPatternFromInstallIfDifferentObject(fs, pattern)
	assert.NoError(t, err)

	assert.True(t, p.IsValid())
	assert.Equal(t, UBootPattern, p.Type)

	version, err := p.Capture(testFile)
	assert.NoError(t, err)
	assert.Equal(t, expectedVersion, version)
}
