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

	"github.com/OSSystems/pkg/log"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/updatehub"
	"github.com/UpdateHub/updatehub/utils"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/afero"
)

type AgentBackend struct {
	*updatehub.UpdateHub
	utils.Rebooter
}

func NewAgentBackend(uh *updatehub.UpdateHub, r utils.Rebooter) (*AgentBackend, error) {
	ab := &AgentBackend{UpdateHub: uh, Rebooter: r}

	return ab, nil
}

func (ab *AgentBackend) Routes() []Route {
	return []Route{
		{Method: "GET", Path: "/info", Handle: ab.info},
		{Method: "GET", Path: "/status", Handle: ab.status},
		{Method: "POST", Path: "/update", Handle: ab.update},
		{Method: "GET", Path: "/update/metadata", Handle: ab.updateMetadata},
		{Method: "POST", Path: "/update/probe", Handle: ab.updateProbe},
		{Method: "POST", Path: "/update/download", Handle: ab.updateDownload},
		{Method: "POST", Path: "/update/download/abort", Handle: ab.updateDownloadAbort},
		{Method: "POST", Path: "/update/install", Handle: ab.updateInstall},
		{Method: "POST", Path: "/reboot", Handle: ab.reboot},
		{Method: "GET", Path: "/log", Handle: ab.log},
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

func (ab *AgentBackend) status(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	out := ab.UpdateHub.State.ToMap()

	outputJSON, _ := json.MarshalIndent(out, "", "    ")

	w.WriteHeader(200)

	fmt.Fprintf(w, string(outputJSON))

	log.Debug(string(outputJSON))
}

func (ab *AgentBackend) update(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	go func() {
		s := updatehub.NewUpdateProbeState()
		ab.UpdateHub.State.Cancel(true, s)
	}()

	w.WriteHeader(202)

	msg := string(`{ "message": "request accepted, update procedure fired" }`)
	fmt.Fprintf(w, msg)

	log.Debug(msg)
}

func (ab *AgentBackend) updateMetadata(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	updateMetadataPath := path.Join(ab.UpdateHub.Settings.UpdateSettings.DownloadDir, metadata.UpdateMetadataFilename)
	data, err := afero.ReadFile(ab.UpdateHub.Store, updateMetadataPath)

	if err != nil {
		w.WriteHeader(400)

		out := map[string]interface{}{}
		out["error"] = err.Error()

		outputJSON, _ := json.MarshalIndent(out, "", "    ")
		fmt.Fprintf(w, string(outputJSON))

		log.Debug(string(outputJSON))
	} else {
		w.WriteHeader(200)
		fmt.Fprintf(w, string(data))
		log.Error(string(data))
	}
}

func (ab *AgentBackend) updateProbe(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	out := map[string]interface{}{}

	um, d := ab.UpdateHub.Controller.ProbeUpdate(0)

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

func (ab *AgentBackend) updateDownload(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	updateMetadataPath := path.Join(ab.UpdateHub.Settings.UpdateSettings.DownloadDir, metadata.UpdateMetadataFilename)
	data, err := afero.ReadFile(ab.UpdateHub.Store, updateMetadataPath)
	if err != nil {
		w.WriteHeader(400)

		out := map[string]interface{}{}
		out["error"] = err.Error()

		outputJSON, _ := json.MarshalIndent(out, "", "    ")
		fmt.Fprintf(w, string(outputJSON))
		log.Error(string(outputJSON))
		return
	}

	um, err := metadata.NewUpdateMetadata(data)
	if err != nil {
		w.WriteHeader(400)

		out := map[string]interface{}{}
		out["error"] = err.Error()

		outputJSON, _ := json.MarshalIndent(out, "", "    ")
		fmt.Fprintf(w, string(outputJSON))
		log.Error(string(outputJSON))
		return
	}

	go func() {
		// cancel the current state and set "downloading" as next
		ab.UpdateHub.State.Cancel(true, updatehub.NewDownloadingState(um, &updatehub.ProgressTrackerImpl{}))
	}()

	w.WriteHeader(202)

	msg := string(`{ "message": "request accepted, downloading update objects" }`)
	fmt.Fprintf(w, msg)

	log.Debug(msg)
}

func (ab *AgentBackend) updateDownloadAbort(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	_, ok := ab.UpdateHub.State.(*updatehub.DownloadingState)
	if !ok {
		w.WriteHeader(400)

		out := map[string]interface{}{}
		out["error"] = "there is no download to be aborted"

		outputJSON, _ := json.MarshalIndent(out, "", "    ")
		fmt.Fprintf(w, string(outputJSON))
		log.Error(string(outputJSON))
		return
	}

	// cancel the current state and set "polling" as next
	ab.UpdateHub.State.Cancel(true, updatehub.NewPollState(ab.UpdateHub.Settings.PollingInterval))

	w.WriteHeader(200)

	msg := string(`{ "message": "request accepted, download aborted" }`)
	fmt.Fprintf(w, msg)

	log.Debug(msg)
}

func (ab *AgentBackend) updateInstall(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	updateMetadataPath := path.Join(ab.UpdateHub.Settings.UpdateSettings.DownloadDir, metadata.UpdateMetadataFilename)
	data, err := afero.ReadFile(ab.UpdateHub.Store, updateMetadataPath)
	if err != nil {
		w.WriteHeader(400)

		out := map[string]interface{}{}
		out["error"] = err.Error()

		outputJSON, _ := json.MarshalIndent(out, "", "    ")
		fmt.Fprintf(w, string(outputJSON))
		log.Error(string(outputJSON))
		return
	}

	um, err := metadata.NewUpdateMetadata(data)
	if err != nil {
		w.WriteHeader(400)

		out := map[string]interface{}{}
		out["error"] = err.Error()

		outputJSON, _ := json.MarshalIndent(out, "", "    ")
		fmt.Fprintf(w, string(outputJSON))
		log.Error(string(outputJSON))
		return
	}

	go func() {
		// cancel the current state and set "installing" as next
		ab.UpdateHub.State.Cancel(true, updatehub.NewInstallingState(um, &updatehub.ProgressTrackerImpl{}, ab.UpdateHub.Store))
	}()

	w.WriteHeader(202)

	msg := string(`{ "message": "request accepted, installing update" }`)
	fmt.Fprintf(w, msg)

	log.Debug(msg)
}

func (ab *AgentBackend) reboot(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	err := ab.Reboot()
	if err != nil {
		w.WriteHeader(400)

		out := map[string]interface{}{}
		out["error"] = err.Error()

		outputJSON, _ := json.MarshalIndent(out, "", "    ")
		fmt.Fprintf(w, string(outputJSON))
		log.Error(string(outputJSON))
		return
	}

	w.WriteHeader(202)

	msg := string(`{ "message": "request accepted, rebooting the device" }`)
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
