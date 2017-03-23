/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package objectmock

import (
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/stretchr/testify/mock"
)

type ObjectMock struct {
	mock.Mock
}

func (om *ObjectMock) GetObjectMetadata() metadata.ObjectMetadata {
	args := om.Called()
	return args.Get(0).(metadata.ObjectMetadata)
}

func (om *ObjectMock) Setup() error {
	args := om.Called()
	return args.Error(0)
}

func (om *ObjectMock) Install() error {
	args := om.Called()
	return args.Error(0)
}

func (om *ObjectMock) Cleanup() error {
	args := om.Called()
	return args.Error(0)
}
