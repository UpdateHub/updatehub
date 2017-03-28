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
	"strings"
)

func MergeErrorList(errorList []error) error {
	if len(errorList) == 0 {
		return nil
	}

	if len(errorList) == 1 {
		return errorList[0]
	}

	errorMessages := []string{}
	for _, err := range errorList {
		errorMessages = append(errorMessages, fmt.Sprintf("(%v)", err))
	}

	return fmt.Errorf("%s", strings.Join(errorMessages[:], "; "))
}
