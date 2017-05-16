/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package server

import (
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type BackendRouter struct {
	backend    Backend
	HTTPRouter *httprouter.Router
}

func NewBackendRouter(b Backend) *BackendRouter {
	br := &BackendRouter{
		backend:    b,
		HTTPRouter: httprouter.New(),
	}

	for _, r := range b.Routes() {
		p, h := br.logMiddleware(r.Path, r.Handle)
		br.HTTPRouter.Handle(r.Method, p, h)
	}

	return br
}

func (br *BackendRouter) logMiddleware(p string, h func(http.ResponseWriter, *http.Request, httprouter.Params)) (string, httprouter.Handle) {
	middleware := func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		br.backend.Logger().Info(fmt.Sprintf("%s %s", r.Method, r.URL))
		h(w, r, p)
	}

	return p, middleware
}
