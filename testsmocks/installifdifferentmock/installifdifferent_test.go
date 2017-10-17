/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package installifdifferentmock

import (
	"fmt"
	"testing"

	"github.com/updatehub/updatehub/installmodes/copy"
	"github.com/updatehub/updatehub/metadata"
	"github.com/stretchr/testify/assert"
)

const (
	ObjectWithInstallIfDifferentSha256Sum = `{
        "mode": "copy",
        "target": "/tmp/dev/xx1",
        "target-type": "device",
        "target-path": "/path",
        "install-if-different": "b5a2c96250612366ea272ffac6d9744aaf4b45aacd96aa7cfcb931ee3b558259"
        }`
)

func TestProceed(t *testing.T) {
	_ = copy.CopyObject{} // just to register the copy object

	expectedError := fmt.Errorf("some error")

	o, err := metadata.NewObjectMetadata([]byte(ObjectWithInstallIfDifferentSha256Sum))
	assert.NoError(t, err)

	iidm := &InstallIfDifferentMock{}
	iidm.On("Proceed", o).Return(true, expectedError)

	b, err := iidm.Proceed(o)

	assert.Equal(t, true, b)
	assert.Equal(t, expectedError, err)

	iidm.AssertExpectations(t)
}
