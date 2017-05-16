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

	"github.com/Sirupsen/logrus"
	"github.com/julienschmidt/httprouter"
)

type AgentBackend struct {
	logger *logrus.Logger
}

func NewAgentBackend(logger *logrus.Logger) (*AgentBackend, error) {
	ab := &AgentBackend{
		logger: logger,
	}

	return ab, nil
}

func (ab *AgentBackend) Logger() *logrus.Logger {
	return ab.logger
}

func (ab *AgentBackend) Routes() []Route {
	return []Route{
		{Method: "GET", Path: "/", Handle: ab.index},
	}
}

func (ab *AgentBackend) index(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	fmt.Fprintf(w, "Agent backend index")
}
