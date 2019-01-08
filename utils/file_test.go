/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package utils

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/UpdateHub/updatehub/testsmocks/filesystemmock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestApplyChmod(t *testing.T) {
	fsb := afero.NewOsFs()

	pdi := &PermissionsDefaultImpl{}

	tempDirPath, err := afero.TempDir(fsb, "", "chmod-test")
	assert.NoError(t, err)
	defer fsb.RemoveAll(tempDirPath)

	filePath := path.Join(tempDirPath, "test-file.txt")

	err = afero.WriteFile(fsb, filePath, []byte("content"), 0644)
	assert.NoError(t, err)

	err = pdi.ApplyChmod(fsb, filePath, "0777")
	assert.NoError(t, err)

	fi, err := fsb.Stat(filePath)
	assert.NoError(t, err)

	assert.Equal(t, os.FileMode(0777), fi.Mode())
}

func TestApplyChmodOnSymlink(t *testing.T) {
	fsb := afero.NewOsFs()

	pdi := &PermissionsDefaultImpl{}

	tempDirPath, err := afero.TempDir(fsb, "", "chmod-test")
	assert.NoError(t, err)
	defer fsb.RemoveAll(tempDirPath)

	filePath := path.Join(tempDirPath, "test-file.txt")

	err = afero.WriteFile(fsb, filePath, []byte("content"), 0644)
	assert.NoError(t, err)

	linkPath := path.Join(tempDirPath, "link")

	err = os.Symlink(filePath, linkPath)
	assert.NoError(t, err)

	err = pdi.ApplyChmod(fsb, linkPath, "0777")
	assert.NoError(t, err)

	fi, err := fsb.Stat(filePath)
	assert.NoError(t, err)

	assert.Equal(t, os.FileMode(0644), fi.Mode())
}

func TestApplyChmodWithParseUintError(t *testing.T) {
	fsb := afero.NewMemMapFs()

	pdi := &PermissionsDefaultImpl{}
	err := pdi.ApplyChmod(fsb, "/dummy", "arty")

	assert.EqualError(t, err, "strconv.ParseUint: parsing \"arty\": invalid syntax")
}

func TestApplyChmodWithEmptyMode(t *testing.T) {
	fsb := afero.NewOsFs()

	pdi := &PermissionsDefaultImpl{}

	tempDirPath, err := afero.TempDir(fsb, "", "chmod-test")
	assert.NoError(t, err)
	defer fsb.RemoveAll(tempDirPath)

	filePath := path.Join(tempDirPath, "test-file.txt")

	err = afero.WriteFile(fsb, filePath, []byte("content"), 0644)
	assert.NoError(t, err)

	err = pdi.ApplyChmod(fsb, filePath, "")
	assert.NoError(t, err)

	fi, err := fsb.Stat(filePath)
	assert.NoError(t, err)

	assert.Equal(t, os.FileMode(0644), fi.Mode())
}

func TestApplyChmodWithChmodError(t *testing.T) {
	afs := afero.NewOsFs()

	pdi := &PermissionsDefaultImpl{}

	tempDirPath, err := afero.TempDir(afs, "", "chmod-test")
	assert.NoError(t, err)
	defer afs.RemoveAll(tempDirPath)

	filePath := path.Join(tempDirPath, "test-file.txt")

	err = afero.WriteFile(afs, filePath, []byte("content"), 0644)
	assert.NoError(t, err)

	fsb := &filesystemmock.FileSystemBackendMock{}
	fsb.On("Chmod", filePath, os.FileMode(0777)).Return(fmt.Errorf("chmod error"))

	err = pdi.ApplyChmod(fsb, filePath, "0777")
	assert.EqualError(t, err, "chmod error")

	fi, err := afs.Stat(filePath)
	assert.NoError(t, err)

	assert.Equal(t, os.FileMode(0644), fi.Mode())
}

func TestApplyChmodWithLstatError(t *testing.T) {
	fsb := afero.NewOsFs()

	pdi := &PermissionsDefaultImpl{}

	tempDirPath, err := afero.TempDir(fsb, "", "chmod-test")
	assert.NoError(t, err)
	defer fsb.RemoveAll(tempDirPath)

	filePath := path.Join(tempDirPath, "test-file.txt")

	err = pdi.ApplyChmod(fsb, filePath, "0777")
	assert.EqualError(t, err, fmt.Sprintf("lstat %s: no such file or directory", filePath))

	fileExists, err := afero.Exists(fsb, filePath)
	assert.False(t, fileExists)
}
