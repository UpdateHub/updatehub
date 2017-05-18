/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package server

import "github.com/julienschmidt/httprouter"

type Route struct {
	Method string
	Path   string
	Handle httprouter.Handle
}

type Backend interface {
	Routes() []Route
}
