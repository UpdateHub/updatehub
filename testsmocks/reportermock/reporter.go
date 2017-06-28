/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package reportermock

import (
	"github.com/UpdateHub/updatehub/client"
	"github.com/stretchr/testify/mock"
)

type ReporterMock struct {
	mock.Mock
}

func (rm *ReporterMock) ReportState(api client.ApiRequester, packageUID string, state string) error {
	args := rm.Called(api, packageUID, state)
	return args.Error(0)
}
