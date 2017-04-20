/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatermock

import (
	"io"
	"time"

	"github.com/UpdateHub/updatehub/client"
	"github.com/stretchr/testify/mock"
)

type UpdaterMock struct {
	mock.Mock
}

func (um *UpdaterMock) CheckUpdate(api client.ApiRequester, data interface{}) (interface{}, time.Duration, error) {
	args := um.Called(api, data)
	return args.Get(0), args.Get(1).(time.Duration), args.Error(2)
}

func (um *UpdaterMock) FetchUpdate(api client.ApiRequester, uri string) (io.ReadCloser, int64, error) {
	args := um.Called(api, uri)
	return args.Get(0).(io.ReadCloser), args.Get(1).(int64), args.Error(2)
}
