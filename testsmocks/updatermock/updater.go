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

	"github.com/stretchr/testify/mock"
	"github.com/updatehub/updatehub/client"
)

type UpdaterMock struct {
	mock.Mock
}

func (um *UpdaterMock) ProbeUpdate(api client.ApiRequester, uri string, data interface{}) (interface{}, []byte, time.Duration, error) {
	args := um.Called(api, uri, data)
	return args.Get(0), args.Get(1).([]byte), args.Get(2).(time.Duration), args.Error(3)
}

func (um *UpdaterMock) DownloadUpdate(api client.ApiRequester, uri string) (io.ReadCloser, int64, error) {
	args := um.Called(api, uri)
	return args.Get(0).(io.ReadCloser), args.Get(1).(int64), args.Error(2)
}
