/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package cmdlinemock

import "github.com/stretchr/testify/mock"

type CmdLineExecuterMock struct {
	mock.Mock
}

func (clm *CmdLineExecuterMock) Execute(cmdline string) ([]byte, error) {
	args := clm.Called(cmdline)
	return args.Get(0).([]byte), args.Error(1)
}
