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
	"path"

	"github.com/UpdateHub/updatehub/updatehub"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/afero"
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
		{Method: "GET", Path: "/status", Handle: ab.status},
		{Method: "POST", Path: "/update", Handle: ab.update},
		{Method: "GET", Path: "/update/metadata", Handle: ab.updateMetadata},
		{Method: "POST", Path: "/update/probe", Handle: ab.updateProbe},
	}
}

func (ab *AgentBackend) info(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	out := map[string]interface{}{}

	out["version"] = ab.UpdateHub.Version
	out["build-time"] = ab.UpdateHub.BuildTime
	out["config"] = ab.UpdateHub.Settings
	out["firmware"] = ab.UpdateHub.FirmwareMetadata

	outputJSON, _ := json.MarshalIndent(out, "", "    ")

	w.WriteHeader(200)

	fmt.Fprintf(w, string(outputJSON))
}

func (ab *AgentBackend) status(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	out := ab.UpdateHub.State.ToMap()

	outputJSON, _ := json.MarshalIndent(out, "", "    ")

	w.WriteHeader(200)

	fmt.Fprintf(w, string(outputJSON))
}

func (ab *AgentBackend) update(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	go func() {
		s := updatehub.NewUpdateCheckState()
		ab.UpdateHub.State.Cancel(true, s)
	}()

	w.WriteHeader(202)

	fmt.Fprintf(w, string(`{ "message": "request accepted, update procedure fired" }`))
}

func (ab *AgentBackend) updateMetadata(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	updateMetadataPath := path.Join(ab.UpdateHub.Settings.UpdateSettings.DownloadDir, updateMetadataFilename)
	data, err := afero.ReadFile(ab.UpdateHub.Store, updateMetadataPath)

	if err != nil {
		w.WriteHeader(400)

		out := map[string]interface{}{}
		out["error"] = err.Error()

		outputJSON, _ := json.MarshalIndent(out, "", "    ")
		fmt.Fprintf(w, string(outputJSON))
	} else {
		w.WriteHeader(200)
		fmt.Fprintf(w, string(data))
	}
}

func (ab *AgentBackend) updateProbe(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	out := map[string]interface{}{}

	um, d := ab.UpdateHub.Controller.CheckUpdate(0)

	if um == nil {
		out["update-available"] = false
	} else {
		out["update-available"] = true
	}

	if d > 0 {
		out["try-again-in"] = d.Seconds()
	}

	w.WriteHeader(200)

	outputJSON, _ := json.MarshalIndent(out, "", "    ")
	fmt.Fprintf(w, string(outputJSON))
}
