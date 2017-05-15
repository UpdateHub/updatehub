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
	"io/ioutil"
	"net/http"
	"os"
	"path"

	"github.com/OSSystems/pkg/log"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/julienschmidt/httprouter"
)

const updateMetadataFilename = "updatemetadata.json"

type ServerBackend struct {
	path           string
	updateMetadata []byte
}

func NewServerBackend(path string) (*ServerBackend, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !stat.IsDir() {
		return nil, fmt.Errorf("%s: not a directory", path)
	}

	sb := &ServerBackend{
		path: path,
	}

	return sb, nil
}

func (sb *ServerBackend) ParseUpdateMetadata() error {
	updateMetadataFilePath := path.Join(sb.path, updateMetadataFilename)

	if _, err := os.Stat(updateMetadataFilePath); err != nil {
		return err
	}

	data, err := ioutil.ReadFile(updateMetadataFilePath)
	if err != nil {
		return err
	}

	um := &metadata.UpdateMetadata{}

	err = json.Unmarshal(data, &um)
	if err != nil {
		return fmt.Errorf("Invalid update metadata: %s", err.Error())
	}

	sb.updateMetadata = data

	return nil
}

func (sb *ServerBackend) Routes() []Route {
	return []Route{
		{Method: "POST", Path: "/upgrades", Handle: sb.getUpdateMetadata},
		{Method: "POST", Path: "/report", Handle: sb.reportStatus},
		{Method: "GET", Path: "/:product/:package/:object", Handle: sb.getObject},
	}
}

func (sb *ServerBackend) getUpdateMetadata(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	if sb.updateMetadata == nil {
		w.WriteHeader(404)
		w.Write([]byte("404 page not found\n"))
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if _, err := w.Write(sb.updateMetadata); err != nil {
		log.Warn(err)
	}
}

func (sb *ServerBackend) reportStatus(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)

	type reportStruct struct {
		Status       string `json:"status"`
		PackageUID   string `json:"package-uid"`
		ErrorMessage string `json:"error-message"`
	}

	var report reportStruct

	err := decoder.Decode(&report)
	if err != nil {
		log.Warn(fmt.Errorf("Invalid report data: %s", err))
		w.WriteHeader(500)
		w.Write([]byte("500 internal server error\n"))
		return
	}

	log.Info(fmt.Sprintf("report: status = %s, package-uid = %s, error-message = %s", report.Status, report.PackageUID, report.ErrorMessage))
}

func (sb *ServerBackend) getObject(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	fileName := path.Join(sb.path, p.ByName("product"), p.ByName("package"), p.ByName("object"))
	http.ServeFile(w, r, fileName)
}
