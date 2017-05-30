/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/UpdateHub/updatehub/updatehub"
	"github.com/julienschmidt/httprouter"
)

type AgentBackend struct {
	*updatehub.UpdateHub
}

func NewAgentBackend(uh *updatehub.UpdateHub) (*AgentBackend, error) {
	ab := &AgentBackend{UpdateHub: uh}

	return ab, nil
}

func (ab *AgentBackend) Routes() []Route {
	return []Route{
		{Method: "GET", Path: "/info", Handle: ab.info},
	}
}

func (ab *AgentBackend) info(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	out := map[string]interface{}{}

	out["version"] = ab.UpdateHub.Version
	out["config"] = ab.UpdateHub.Settings
	out["firmware"] = ab.UpdateHub.FirmwareMetadata

	outputJSON, _ := json.MarshalIndent(out, "", "    ")

	fmt.Fprintf(w, string(outputJSON))
}
