/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package progresstrackermock

import "github.com/stretchr/testify/mock"

type ProgressTrackerMock struct {
	mock.Mock
}

func (ptm *ProgressTrackerMock) SetProgress(progress int) {
	ptm.Called(progress)
}

func (ptm *ProgressTrackerMock) GetProgress() int {
	args := ptm.Called()
	return args.Int(0)
}
