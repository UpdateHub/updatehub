/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package responsewritermock

import (
	"net/http"

	"github.com/stretchr/testify/mock"
)

type ResponseWriterMock struct {
	mock.Mock
	http.ResponseWriter
}

func (rwm *ResponseWriterMock) Header() http.Header {
	args := rwm.Called()
	return args.Get(0).(http.Header)
}

func (rwm *ResponseWriterMock) Write(b []byte) (int, error) {
	args := rwm.Called(b)
	return args.Int(0), args.Error(1)
}

func (rwm *ResponseWriterMock) WriteHeader(h int) {
	rwm.Called(h)
}
