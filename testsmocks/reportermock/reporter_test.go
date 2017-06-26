/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package reportermock

import (
	"fmt"
	"testing"

	"github.com/UpdateHub/updatehub/client"
	"github.com/stretchr/testify/assert"
)

func TestReportState(t *testing.T) {
	rm := &ReporterMock{}
	ar := client.NewApiClient("server_address").Request()

	rm.On("ReportState", ar, "sha256sum1", "idle").Return(nil).Once()
	err := rm.ReportState(ar, "sha256sum1", "idle")
	assert.NoError(t, err)

	rm.On("ReportState", ar, "sha256sum2", "downloading").Return(fmt.Errorf("report error")).Once()
	err = rm.ReportState(ar, "sha256sum2", "downloading")
	assert.EqualError(t, err, "report error")

	rm.AssertExpectations(t)
}
