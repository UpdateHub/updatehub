/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package copymock

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/UpdateHub/updatehub/testsmocks/filesystemmock"
)

func TestCopyFile(t *testing.T) {
	expectedError := fmt.Errorf("some error")
	chunkSize := 256
	skip := 2
	seek := 3
	count := 5
	truncate := true
	compressed := false

	fsm := &filesystemmock.FileSystemBackendMock{}
	cm := &CopyMock{}
	cm.On("CopyFile", fsm, nil, "sourcepath", "targetpath", chunkSize, skip, seek, count, truncate, compressed).Return(expectedError)

	err := cm.CopyFile(fsm, nil, "sourcepath", "targetpath", chunkSize, skip, seek, count, truncate, compressed)

	assert.Equal(t, expectedError, err)

	cm.AssertExpectations(t)
	fsm.AssertExpectations(t)
}

func TestCopyToProcessStdin(t *testing.T) {
	expectedError := fmt.Errorf("some error")
	compressed := false

	fsm := &filesystemmock.FileSystemBackendMock{}
	cm := &CopyMock{}
	cm.On("CopyToProcessStdin", fsm, nil, "sourcepath", "cmdline", compressed).Return(expectedError)

	err := cm.CopyToProcessStdin(fsm, nil, "sourcepath", "cmdline", compressed)

	assert.Equal(t, expectedError, err)

	cm.AssertExpectations(t)
	fsm.AssertExpectations(t)
}

func TestCopy(t *testing.T) {
	expectedError := fmt.Errorf("some error")
	wr := ioutil.Discard
	rd := strings.NewReader("")
	chunkSize := 256
	skip := 2
	count := 5
	compressed := false
	cancel := make(<-chan bool)
	timeout := time.Minute

	fsm := &filesystemmock.FileSystemBackendMock{}
	cm := &CopyMock{}
	cm.On("Copy", wr, rd, timeout, cancel, chunkSize, skip, count, compressed).Return(false, expectedError)

	b, err := cm.Copy(wr, rd, timeout, cancel, chunkSize, skip, count, compressed)

	assert.Equal(t, false, b)
	assert.Equal(t, expectedError, err)

	cm.AssertExpectations(t)
	fsm.AssertExpectations(t)
}
