/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeErrorList(t *testing.T) {
	var errorList []error

	err := MergeErrorList(errorList)
	assert.NoError(t, err)

	errorList = append(errorList, fmt.Errorf("first error"))

	err = MergeErrorList(errorList)
	assert.EqualError(t, err, "first error")

	errorList = append(errorList, fmt.Errorf("second error"))
	errorList = append(errorList, fmt.Errorf("third error"))

	err = MergeErrorList(errorList)
	assert.EqualError(t, err, "(first error); (second error); (third error)")
}
