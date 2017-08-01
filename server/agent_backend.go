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
	"sync"

	"github.com/OSSystems/pkg/log"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/updatehub"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/afero"
)

type AgentBackend struct {
	*updatehub.UpdateHub

	requestsInProgress      int
	requestsInProgressMutex sync.Mutex
	AllRequestsFinished     chan bool
}

func NewAgentBackend(uh *updatehub.UpdateHub) (*AgentBackend, error) {
	ab := &AgentBackend{UpdateHub: uh}
	ab.AllRequestsFinished = make(chan bool, 1)

	return ab, nil
}

func (ab *AgentBackend) increaseRequestsCount() {
	ab.requestsInProgressMutex.Lock()
	ab.requestsInProgress++
	ab.requestsInProgressMutex.Unlock()
}

func (ab *AgentBackend) decreaseRequestsCount() {
	ab.requestsInProgressMutex.Lock()
	ab.requestsInProgress--
	ab.requestsInProgressMutex.Unlock()

	var allFinished bool
	if ab.requestsInProgress == 0 {
		allFinished = true
	} else {
		allFinished = false
	}

	// "non-blocking" write to channel
	select {
	case ab.AllRequestsFinished <- allFinished:
	default:
	}
}

func (ab *AgentBackend) Routes() []Route {
	routes := []Route{
		{Method: "GET", Path: "/info", Handle: ab.info},
		{Method: "GET", Path: "/status", Handle: ab.status},
		{Method: "GET", Path: "/update/metadata", Handle: ab.updateMetadata},
		{Method: "POST", Path: "/update/probe", Handle: ab.updateProbe},
		{Method: "GET", Path: "/log", Handle: ab.log},
	}

	if ab.Settings.ManualMode {
		routes = append(routes, Route{Method: "POST", Path: "/update", Handle: ab.update})
		routes = append(routes, Route{Method: "POST", Path: "/update/download", Handle: ab.updateDownload})
		routes = append(routes, Route{Method: "POST", Path: "/update/download/abort", Handle: ab.updateDownloadAbort})
		routes = append(routes, Route{Method: "POST", Path: "/update/install", Handle: ab.updateInstall})
		routes = append(routes, Route{Method: "POST", Path: "/reboot", Handle: ab.reboot})
	}

	return routes
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
	out := ab.UpdateHub.GetState().ToMap()

	outputJSON, _ := json.MarshalIndent(out, "", "    ")

	w.WriteHeader(200)

	fmt.Fprintf(w, string(outputJSON))

	log.Debug(string(outputJSON))
}

func (ab *AgentBackend) update(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ab.UpdateHub.SetState(updatehub.NewUpdateProbeState())

	ab.increaseRequestsCount()
	go func() {
		ab.UpdateHub.ProcessCurrentState()
		ab.decreaseRequestsCount()
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

		ab.UpdateHub.SetState(updatehub.NewErrorState(nil, updatehub.NewTransientError(err)))
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

		ab.UpdateHub.SetState(updatehub.NewErrorState(nil, updatehub.NewTransientError(err)))
		return
	}

	ab.UpdateHub.SetState(updatehub.NewDownloadingState(um, &updatehub.ProgressTrackerImpl{}))

	ab.increaseRequestsCount()
	go func() {
		ab.UpdateHub.ProcessCurrentState()
		ab.decreaseRequestsCount()
	}()

	w.WriteHeader(202)

	msg := string(`{ "message": "request accepted, downloading update objects" }`)
	fmt.Fprintf(w, msg)

	log.Debug(msg)
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
	ab.UpdateHub.SetState(ab.UpdateHub.ProcessCurrentState())

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

		ab.UpdateHub.SetState(updatehub.NewErrorState(nil, updatehub.NewTransientError(err)))
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

		ab.UpdateHub.SetState(updatehub.NewErrorState(nil, updatehub.NewTransientError(err)))
		return
	}

	ab.UpdateHub.SetState(updatehub.NewInstallingState(um, &updatehub.ProgressTrackerImpl{}, ab.UpdateHub.Store))

	ab.increaseRequestsCount()
	go func() {
		ab.UpdateHub.ProcessCurrentState()
		ab.decreaseRequestsCount()
	}()

	w.WriteHeader(202)

	msg := string(`{ "message": "request accepted, installing update" }`)
	fmt.Fprintf(w, msg)

	log.Debug(msg)
}

func (ab *AgentBackend) reboot(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ab.UpdateHub.SetState(updatehub.NewRebootState())

	ab.increaseRequestsCount()
	go func() {
		ab.UpdateHub.ProcessCurrentState()
		ab.decreaseRequestsCount()
	}()

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
