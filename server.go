/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type Server struct {
	ServerController

	uh   *UpdateHub
	router *httprouter.Router
}

type ServerController interface {
	Index(http.ResponseWriter, *http.Request, httprouter.Params)
}

func (s *Server) CreateRouter() {
	s.router = httprouter.New()
	s.router.GET("/", s.ServerController.Index)
}

func (s *Server) Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
}

func NewServer(uh *UpdateHub) *Server {
	s := &Server{
		uh: uh,
	}

	s.ServerController = s

	return s
}
