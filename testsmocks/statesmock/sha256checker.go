/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package statesmock

import (
	"github.com/spf13/afero"
	"github.com/stretchr/testify/mock"
)

type Sha256CheckerMock struct {
	mock.Mock
}

func (scm *Sha256CheckerMock) CheckDownloadedObjectSha256sum(fsBackend afero.Fs, downloadDir string, expectedSha256sum string) (bool, error) {
	args := scm.Called(fsBackend, downloadDir, expectedSha256sum)
	return args.Bool(0), args.Error(1)
}
