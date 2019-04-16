/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/OSSystems/pkg/log"
	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/updatehub"
	"github.com/UpdateHub/updatehub/utils"
)

type AgentBackend struct {
	*updatehub.UpdateHub
}

func NewAgentBackend(uh *updatehub.UpdateHub) (*AgentBackend, error) {
	ab := &AgentBackend{UpdateHub: uh}

	return ab, nil
}

func (ab *AgentBackend) info(w http.ResponseWriter, r *http.Request) {
	out := map[string]interface{}{}

	out["version"] = ab.UpdateHub.Version
	out["config"] = ab.UpdateHub.Settings
	out["firmware"] = ab.UpdateHub.FirmwareMetadata

	outputJSON, _ := json.MarshalIndent(out, "", "    ")

	w.WriteHeader(200)

	fmt.Fprintf(w, string(outputJSON))

	log.Debug(string(outputJSON))
}

func (ab *AgentBackend) probe(w http.ResponseWriter, r *http.Request) {
	apiClient := ab.UpdateHub.DefaultApiClient

	var in struct {
		From            string `json:"from"`
		ServerAddress   string `json:"server-address"`
		IgnoreProbeASAP bool   `json:"ignore-probe-asap"`
	}

	buffer := new(bytes.Buffer)
	buffer.ReadFrom(r.Body)
	body := buffer.Bytes()

	err := json.Unmarshal(body, &in)
	if err != nil {
		log.Warn("failed to parse a /probe request: ", err)
	}

	// ServerAddress is deprecated, so this was neccessary in order to maintain compability
	in.From = in.ServerAddress

	if in.From != "" {
		target, err := url.Parse(in.From)
		if err != nil {
		}

		var updatePackage *UpdatePackage

		switch target.Scheme {
		case "http":
			fallthrough
		case "https":
			if strings.HasSuffix(target.Path, ".uhupkg") {
				updatePackage, err = fetchUpdatePackage(target)
				if err != nil {
					log.Error(err)
					w.WriteHeader(500)
					return
				}
			} else {
				sanitizedAddress, err := utils.SanitizeServerAddress(in.From)

				if err != nil {
					log.Warn("failed to sanitize a server address from /probe request: ", err)
				} else {
					apiClient = client.NewApiClient(sanitizedAddress)
				}
			}
		case "file":
			if fi, err := os.Stat(target.Path); err == nil {
				var filename string

				if fi.IsDir() {
					files, _ := ioutil.ReadDir(target.Path)
					sort.Slice(files, func(i, j int) bool {
						return files[i].ModTime().After(files[j].ModTime())
					})

					for _, f := range files {
						if strings.HasSuffix(f.Name(), ".uhupkg") {
							filename = f.Name()
							break
						}
					}

					if fi == nil {
						w.WriteHeader(500)
						return
					}
				} else {
					filename = fi.Name()
				}

				if filename != "" {
					f, _ := os.Open(filename)
					defer f.Close()

					updatePackage, err = NewUpdatePackage(f)
					if err != nil {
						w.WriteHeader(500)
						return
					}
				} else {
					w.WriteHeader(500)
					return
				}
			} else {
				w.WriteHeader(500)
				return
			}
		}

		if updatePackage != nil {
			s, err := NewLocalServer(updatePackage)
			if err != nil {
				log.Error(err)
				w.WriteHeader(500)
				return
			}

			apiClient = client.NewApiClient(fmt.Sprintf("http://localhost:%d", s.port))

			go func() {
				err := s.start()
				log.Fatal(err)
			}()

			if ok := s.waitForAvailable(); !ok {
				w.WriteHeader(500)
				return
			}
		}
	}

	out := map[string]interface{}{}

	switch state := ab.UpdateHub.GetState().(type) {
	case *updatehub.IdleState, *updatehub.PollState, *updatehub.ProbeState:
		ab.UpdateHub.IgnoreProbeASAP = in.IgnoreProbeASAP

		s := updatehub.NewProbeState(apiClient)

		ab.UpdateHub.Cancel(s)

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
	default:
		out["busy"] = true
		out["current-state"] = updatehub.StateToString(state.ID())

		w.WriteHeader(202)
	}

	outputJSON, _ := json.MarshalIndent(out, "", "    ")
	fmt.Fprintf(w, string(outputJSON))

	log.Debug(string(outputJSON))
}

func (ab *AgentBackend) updateDownloadAbort(w http.ResponseWriter, r *http.Request) {
	_, ok := ab.UpdateHub.GetState().(*updatehub.DownloadingState)
	if !ok {
		w.WriteHeader(400)

		out := map[string]interface{}{}
		out["error"] = "there is no download to be aborted"

		outputJSON, _ := json.MarshalIndent(out, "", "    ")
		fmt.Fprintf(w, string(outputJSON))
		log.Error(string(outputJSON))

		ab.UpdateHub.SetState(updatehub.NewErrorState(ab.UpdateHub.DefaultApiClient, nil, updatehub.NewTransientError(fmt.Errorf("there is no download to be aborted"))))
		return
	}

	ab.UpdateHub.Cancel(updatehub.NewIdleState())

	w.WriteHeader(200)

	msg := string(`{ "message": "request accepted, download aborted" }`)
	fmt.Fprintf(w, msg)

	log.Debug(msg)
}

func (ab *AgentBackend) log(w http.ResponseWriter, r *http.Request) {
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
