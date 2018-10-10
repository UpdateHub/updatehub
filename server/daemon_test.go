/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package server

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/libarchivemock"
)

func TestNewDaemon(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	d, err := NewDaemon(sb)
	assert.NoError(t, err)

	assert.NotNil(t, d.fswatcher)
	assert.Equal(t, sb, d.backend)

	lam.AssertExpectations(t)
}

func TestNewDaemonWithNonExistantPath(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)

	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	os.RemoveAll(testPath)

	d, err := NewDaemon(sb)
	assert.EqualError(t, err, "no such file or directory")

	assert.Nil(t, d)

	lam.AssertExpectations(t)
}

func TestRunWithWriteNotification(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	d, err := NewDaemon(sb)
	assert.NoError(t, err)

	go d.Run()
	<-d.started

	updateMetadataFilePath := path.Join(testPath, metadata.UpdateMetadataFilename)
	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	<-d.metadataWritten

	assert.Equal(t, []byte(ValidJSONMetadata), d.backend.selectedPackage.updateMetadata)

	lam.AssertExpectations(t)
}

func TestRunWithRemoveNotification(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)

	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	d, err := NewDaemon(sb)
	assert.NoError(t, err)

	go d.Run()

	time.Sleep(100 * time.Millisecond)

	// remove the directory (this test must ensure that the notifier
	// doesn't monitor testPath anymore after the removal)
	os.RemoveAll(testPath)

	err = os.MkdirAll(testPath, 0777)
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, metadata.UpdateMetadataFilename)
	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, (*SelectedPackage)(nil), d.backend.selectedPackage)

	lam.AssertExpectations(t)
}
