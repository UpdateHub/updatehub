/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package fileinfomock

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestName(t *testing.T) {
	expected := "name"

	fim := &FileInfoMock{}
	fim.On("Name").Return(expected)

	assert.Equal(t, expected, fim.Name())

	fim.AssertExpectations(t)
}

func TestSize(t *testing.T) {
	expected := 8

	fim := &FileInfoMock{}
	fim.On("Size").Return(int64(expected))

	assert.Equal(t, int64(expected), fim.Size())

	fim.AssertExpectations(t)
}

func TestMode(t *testing.T) {
	expected := os.FileMode(0666)

	fim := &FileInfoMock{}
	fim.On("Mode").Return(expected)

	assert.Equal(t, expected, fim.Mode())

	fim.AssertExpectations(t)
}

func TestModTime(t *testing.T) {
	expected := time.Now()

	fim := &FileInfoMock{}
	fim.On("ModTime").Return(expected)

	assert.Equal(t, expected, fim.ModTime())

	fim.AssertExpectations(t)
}

func TestIsDir(t *testing.T) {
	expected := true

	fim := &FileInfoMock{}
	fim.On("IsDir").Return(expected)

	assert.Equal(t, expected, fim.IsDir())

	fim.AssertExpectations(t)
}

func TestSys(t *testing.T) {
	expected := "hello"

	fim := &FileInfoMock{}
	fim.On("Sys").Return(expected)

	assert.Equal(t, expected, fim.Sys())

	fim.AssertExpectations(t)
}
