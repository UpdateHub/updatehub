package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type Server struct {
	ServerController

	fota   *EasyFota
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

func NewServer(fota *EasyFota) *Server {
	s := &Server{
		fota: fota,
	}

	s.ServerController = s

	return s
}
