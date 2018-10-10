/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package reportermock

import (
	"github.com/stretchr/testify/mock"
	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/metadata"
)

type ReporterMock struct {
	mock.Mock
}

func (rm *ReporterMock) ReportState(api client.ApiRequester, packageUID string, previousState string, state string, errorMessage string, fm metadata.FirmwareMetadata) error {
	args := rm.Called(api, packageUID, previousState, state, errorMessage, fm)
	return args.Error(0)
}
