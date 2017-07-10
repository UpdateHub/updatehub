/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package progresstrackermock

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetProgress(t *testing.T) {
	ptm := &ProgressTrackerMock{}
	ptm.On("SetProgress", 33)

	ptm.SetProgress(33)

	ptm.AssertExpectations(t)
}

func TestGetProgress(t *testing.T) {
	ptm := &ProgressTrackerMock{}
	ptm.On("GetProgress").Return(33)

	progress := ptm.GetProgress()

	assert.Equal(t, 33, progress)

	ptm.AssertExpectations(t)
}
