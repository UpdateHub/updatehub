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
	"os"
	"path"
	"strings"

	"github.com/OSSystems/pkg/log"
	"github.com/julienschmidt/httprouter"
	"github.com/updatehub/updatehub/libarchive"
	"github.com/updatehub/updatehub/metadata"
	"github.com/updatehub/updatehub/utils"
)

type SelectedPackage struct {
	updateMetadata []byte
	signature      []byte
	uhupkgPath     string
}

type ServerBackend struct {
	path            string
	selectedPackage *SelectedPackage
	LibArchive      libarchive.API
}

func NewServerBackend(la libarchive.API, path string) (*ServerBackend, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !stat.IsDir() {
		finalErr := fmt.Errorf("%s: not a directory", path)
		log.Error(finalErr)
		return nil, finalErr
	}

	sb := &ServerBackend{
		path:            path,
		selectedPackage: nil,
		LibArchive:      la,
	}

	return sb, nil
}

func (sb *ServerBackend) parseUpdateMetadata() ([]byte, error) {
	updateMetadataFilePath := path.Join(sb.path, metadata.UpdateMetadataFilename)

	if _, err := os.Stat(updateMetadataFilePath); err != nil {
		return nil, err
	}

	data, err := ioutil.ReadFile(updateMetadataFilePath)
	if err != nil {
		return nil, err
	}

	um := &metadata.UpdateMetadata{}

	err = json.Unmarshal(data, &um)
	if err != nil {
		finalErr := fmt.Errorf("Invalid update metadata: %s", err.Error())
		log.Error(finalErr)
		return nil, finalErr
	}

	return data, nil
}

func (sb *ServerBackend) parseUhuPkg(pkgpath string) ([]byte, []byte, error) {
	reader, err := libarchive.NewReader(sb.LibArchive, pkgpath, 10240)
	if err != nil {
		return nil, nil, err
	}

	metadataBuff := bytes.NewBuffer(nil)
	err = reader.ExtractFile("metadata", metadataBuff)
	if err != nil {
		return nil, nil, err
	}

	reader, err = libarchive.NewReader(sb.LibArchive, pkgpath, 10240)
	if err != nil {
		return metadataBuff.Bytes(), nil, err
	}

	signatureBuff := bytes.NewBuffer(nil)
	err = reader.ExtractFile("signature", signatureBuff)
	if err != nil {
		return metadataBuff.Bytes(), nil, err
	}

	return metadataBuff.Bytes(), signatureBuff.Bytes(), nil
}

func (sb *ServerBackend) ProcessDirectory() error {
	var err error
	var packagesFound []string

	files, _ := ioutil.ReadDir(sb.path)
	for _, f := range files {
		if f.Name() == metadata.UpdateMetadataFilename {
			packagesFound = append(packagesFound, metadata.UpdateMetadataFilename)
		}

		if strings.HasSuffix(f.Name(), ".uhupkg") {
			packagesFound = append(packagesFound, f.Name())
		}
	}

	if len(packagesFound) > 1 {
		finalErr := fmt.Errorf("the path provided must not have more than 1 package. Found: %d", len(packagesFound))
		log.Error(finalErr)
		return finalErr
	}

	pkgpath := path.Join(sb.path, packagesFound[0])

	var updateMetadata []byte
	var signature []byte
	var uhupkgPath string

	log.Info("selected package: ", pkgpath)
	if packagesFound[0] == metadata.UpdateMetadataFilename {
		updateMetadata, err = sb.parseUpdateMetadata()
	} else {
		uhupkgPath = pkgpath
		updateMetadata, signature, err = sb.parseUhuPkg(pkgpath)
	}

	if err != nil {
		return err
	}

	p := &SelectedPackage{
		updateMetadata: updateMetadata,
		signature:      signature,
		uhupkgPath:     uhupkgPath,
	}

	sb.selectedPackage = p

	log.Info("selected package-uid: ", utils.DataSha256sum(updateMetadata))
	log.Debug("update metadata loaded: \n", string(updateMetadata))

	return nil
}

func (sb *ServerBackend) Routes() []Route {
	return []Route{
		{Method: "POST", Path: "/upgrades", Handle: sb.getUpdateMetadata},
		{Method: "POST", Path: "/report", Handle: sb.reportStatus},
		{Method: "GET", Path: "/products/:product/packages/:package/objects/:object", Handle: sb.getObject},
	}
}

func (sb *ServerBackend) getUpdateMetadata(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	if sb.selectedPackage == nil || sb.selectedPackage.updateMetadata == nil {
		w.WriteHeader(404)
		w.Write([]byte("404 page not found\n"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("UH-Signature", string(sb.selectedPackage.signature))

	if _, err := w.Write(sb.selectedPackage.updateMetadata); err != nil {
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
	if sb.selectedPackage == nil {
		log.Error("no package selected yet")
		w.WriteHeader(500)
		fmt.Fprintf(w, "500 internal server error\n")
		return
	}

	if sb.selectedPackage.uhupkgPath != "" {
		// is uhupkg in the directory

		fileName := p.ByName("object")

		// package was already parsed, we can safely ignore the error here
		reader, _ := libarchive.NewReader(sb.LibArchive, sb.selectedPackage.uhupkgPath, 10240)
		defer reader.Free()

		err := reader.ExtractFile(fileName, w)
		if err != nil {
			log.Error(err)
			w.WriteHeader(500)
			fmt.Fprintf(w, "500 internal server error\n")
			return
		}
	} else {
		// is updatemetadata.json in the directory

		fileName := path.Join(sb.path, p.ByName("product"), p.ByName("package"), p.ByName("object"))

		if r.Method == http.MethodGet {
			http.ServeFile(w, r, fileName)
		}
	}
}
