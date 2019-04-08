package server

import (
	"net/http"

	"github.com/gorilla/mux"
)

type Router struct {
	router  *mux.Router
	backend *AgentBackend
}

func NewRouter(backend *AgentBackend) *Router {
	s := &Router{router: mux.NewRouter(), backend: backend}

	s.router.HandleFunc("/info", backend.info).Methods("GET")
	s.router.HandleFunc("/log", backend.log).Methods("GET")
	s.router.HandleFunc("/probe", backend.probe).Methods("POST")
	s.router.HandleFunc("/update/download/abort", backend.updateDownloadAbort).Methods("POST")

	return s
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}
