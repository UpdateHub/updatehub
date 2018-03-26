/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package activeinactivemock

import "github.com/stretchr/testify/mock"

type ActiveInactiveMock struct {
	mock.Mock
}

func (aim *ActiveInactiveMock) Active() (int, error) {
	args := aim.Called()
	return args.Int(0), args.Error(1)
}

func (aim *ActiveInactiveMock) SetActive(active int) error {
	args := aim.Called(active)
	return args.Error(0)
}

func (aim *ActiveInactiveMock) SetValidate() error {
	args := aim.Called()
	return args.Error(0)
}
