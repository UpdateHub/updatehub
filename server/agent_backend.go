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

	downloadCancelChan   chan bool
	downloadProgressChan chan int
	installProgressChan  chan int
}

func NewAgentBackend(uh *updatehub.UpdateHub, r utils.Rebooter) (*AgentBackend, error) {
	ab := &AgentBackend{UpdateHub: uh, Rebooter: r}

	ab.downloadCancelChan = make(chan bool)
	ab.downloadProgressChan = make(chan int)

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

func (ab *AgentBackend) updateDownload(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	updateMetadataPath := path.Join(ab.UpdateHub.Settings.UpdateSettings.DownloadDir, updateMetadataFilename)
	data, err := afero.ReadFile(ab.UpdateHub.Store, updateMetadataPath)
	if err != nil {
		w.WriteHeader(400)

		out := map[string]interface{}{}
		out["error"] = err.Error()

		outputJSON, _ := json.MarshalIndent(out, "", "    ")
		fmt.Fprintf(w, string(outputJSON))
		return
	}

	um, err := metadata.NewUpdateMetadata(data)
	if err != nil {
		w.WriteHeader(400)

		out := map[string]interface{}{}
		out["error"] = err.Error()

		outputJSON, _ := json.MarshalIndent(out, "", "    ")
		fmt.Fprintf(w, string(outputJSON))
		return
	}

	go func() {
		ab.UpdateHub.Controller.FetchUpdate(um, ab.downloadCancelChan, ab.downloadProgressChan)
	}()

	w.WriteHeader(202)

	fmt.Fprintf(w, string(`{ "message": "request accepted, downloading update objects" }`))
}

func (ab *AgentBackend) updateDownloadAbort(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// FIXME: how to test this?
	// ab.downloadCancelChan <- true

	w.WriteHeader(400)

	out := map[string]interface{}{}
	out["error"] = "not yet implemented"

	outputJSON, _ := json.MarshalIndent(out, "", "    ")
	fmt.Fprintf(w, string(outputJSON))
}

func (ab *AgentBackend) updateInstall(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	updateMetadataPath := path.Join(ab.UpdateHub.Settings.UpdateSettings.DownloadDir, updateMetadataFilename)
	data, err := afero.ReadFile(ab.UpdateHub.Store, updateMetadataPath)
	if err != nil {
		w.WriteHeader(400)

		out := map[string]interface{}{}
		out["error"] = err.Error()

		outputJSON, _ := json.MarshalIndent(out, "", "    ")
		fmt.Fprintf(w, string(outputJSON))
		return
	}

	um, err := metadata.NewUpdateMetadata(data)
	if err != nil {
		w.WriteHeader(400)

		out := map[string]interface{}{}
		out["error"] = err.Error()

		outputJSON, _ := json.MarshalIndent(out, "", "    ")
		fmt.Fprintf(w, string(outputJSON))
		return
	}

	go func() {
		ab.UpdateHub.Controller.InstallUpdate(um, ab.installProgressChan)
	}()

	w.WriteHeader(202)

	fmt.Fprintf(w, string(`{ "message": "request accepted, installing update" }`))
}

func (ab *AgentBackend) reboot(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	err := ab.Reboot()
	if err != nil {
		w.WriteHeader(400)

		out := map[string]interface{}{}
		out["error"] = err.Error()

		outputJSON, _ := json.MarshalIndent(out, "", "    ")
		fmt.Fprintf(w, string(outputJSON))
		return
	}

	w.WriteHeader(202)

	fmt.Fprintf(w, string(`{ "message": "request accepted, rebooting the device" }`))
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
}
