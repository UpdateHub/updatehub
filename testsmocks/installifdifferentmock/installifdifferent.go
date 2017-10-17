/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package installifdifferentmock

import (
	"github.com/updatehub/updatehub/metadata"
	"github.com/stretchr/testify/mock"
)

type InstallIfDifferentMock struct {
	mock.Mock
}

func (iidm *InstallIfDifferentMock) Proceed(o metadata.Object) (bool, error) {
	args := iidm.Called(o)
	return args.Bool(0), args.Error(1)
}
