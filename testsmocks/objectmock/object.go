/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package objectmock

import (
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/stretchr/testify/mock"
)

type ObjectMock struct {
	metadata.ObjectMetadata
	mock.Mock
}

func (om *ObjectMock) Setup() error {
	args := om.Called()
	return args.Error(0)
}

func (om *ObjectMock) Install(downloadDir string) error {
	args := om.Called(downloadDir)
	return args.Error(0)
}

func (om *ObjectMock) Cleanup() error {
	args := om.Called()
	return args.Error(0)
}
