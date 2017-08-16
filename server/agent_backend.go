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

	"github.com/OSSystems/pkg/log"
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
		{Method: "GET", Path: "/log", Handle: ab.log},
		{Method: "POST", Path: "/probe", Handle: ab.probe},
		{Method: "POST", Path: "/update/download/abort", Handle: ab.updateDownloadAbort},
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

	log.Debug(string(outputJSON))
}

func (ab *AgentBackend) probe(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	out := map[string]interface{}{}

	s := updatehub.NewProbeState()
	go func() {
		ab.UpdateHub.Cancel(s)
	}()

	<-s.ProbeResponseReady

	um, d := s.ProbeResponse()

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

	log.Debug(string(outputJSON))
}

func (ab *AgentBackend) updateDownloadAbort(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	_, ok := ab.UpdateHub.GetState().(*updatehub.DownloadingState)
	if !ok {
		w.WriteHeader(400)

		out := map[string]interface{}{}
		out["error"] = "there is no download to be aborted"

		outputJSON, _ := json.MarshalIndent(out, "", "    ")
		fmt.Fprintf(w, string(outputJSON))
		log.Error(string(outputJSON))

		ab.UpdateHub.SetState(updatehub.NewErrorState(nil, updatehub.NewTransientError(fmt.Errorf("there is no download to be aborted"))))
		return
	}

	ab.UpdateHub.Cancel(updatehub.NewIdleState())

	w.WriteHeader(200)

	msg := string(`{ "message": "request accepted, download aborted" }`)
	fmt.Fprintf(w, msg)

	log.Debug(msg)
}

func (ab *AgentBackend) log(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	out := []map[string]interface{}{}

	for _, e := range log.AllEntries() {
		out = append(out, map[string]interface{}{
			"message": e.Message,
			"level":   string(e.Level.String()),
			"time":    string(e.Time.String()),
			"data":    e.Data,
		})
	}

	w.WriteHeader(200)

	outputJSON, _ := json.MarshalIndent(out, "", "    ")
	fmt.Fprintf(w, string(outputJSON))

	log.Debug(string(outputJSON))
}
