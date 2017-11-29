/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package statesmock

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestCheckDownloadedObjectSha256sum(t *testing.T) {
	fs := afero.NewMemMapFs()
	expectedResult := true
	expectedError := fmt.Errorf("some error")

	scm := &Sha256CheckerMock{}
	scm.On("CheckDownloadedObjectSha256sum", fs, "downloaddir", "sha256sum").Return(expectedResult, expectedError)

	ok, err := scm.CheckDownloadedObjectSha256sum(fs, "downloaddir", "sha256sum")

	assert.Equal(t, expectedResult, ok)
	assert.Equal(t, expectedError, err)

	scm.AssertExpectations(t)
}
