/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package rebootermock

import (
	"github.com/stretchr/testify/mock"
)

type RebooterMock struct {
	mock.Mock
}

func (rm *RebooterMock) Reboot() error {
	args := rm.Called()
	return args.Error(0)
}
