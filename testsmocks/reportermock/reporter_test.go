/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package reportermock

import (
	"fmt"
	"testing"

	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/stretchr/testify/assert"
)

func TestReportState(t *testing.T) {
	rm := &ReporterMock{}
	ar := client.NewApiClient("server_address").Request()

	fm := metadata.FirmwareMetadata{
		ProductUID: "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
		DeviceIdentity: map[string]string{
			"id1": "value1",
		},
		Hardware: "board",
		Version:  "2.2",
	}

	rm.On("ReportState", ar, "sha256sum1", "installed", "idle", "", fm).Return(nil).Once()
	err := rm.ReportState(ar, "sha256sum1", "installed", "idle", "", fm)
	assert.NoError(t, err)

	rm.On("ReportState", ar, "sha256sum2", "poll", "downloading", "", fm).Return(fmt.Errorf("report error")).Once()
	err = rm.ReportState(ar, "sha256sum2", "poll", "downloading", "", fm)
	assert.EqualError(t, err, "report error")

	rm.AssertExpectations(t)
}
