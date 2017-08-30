/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

/*

//FIXME: remove this later

- update metadata upload
-- fixed server route "/packages" to receive the metadata (POST)
-- expect a http header 'UH-SIGNATURE' containing the metadata signature
-- when successful, send as response the packageuid. json with a "uid" key

- client asks server to get the url which the objects will be uploaded
  to
-- expect a POST at route '/packages/<packageuid>/objects/<objuid>',
   response is anything but 200 (200 means file already exists on server):
   json with fields 'storage' containing "dummy" and 'url' containing
   the url to which the files will be uploaded

- client uploads objects
-- expect a PUT at the url sent to the client
-- the PUT will come with Content-Length set and the connection open
   waiting for streamed reading

- client ends transaction
-- expect a PUT at '/packages/<packageuid>/finish'



-- error cases are responded with a json field 'error_message'

*/

package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/OSSystems/pkg/log"
	"github.com/UpdateHub/updatehub/libarchive"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/utils"
	"github.com/julienschmidt/httprouter"
)

type Package struct {
	updateMetadata     []byte
	signature          []byte
	path               string
	isUhupkgSingleFile bool
}

type ServerBackend struct {
	selectedPackage         *Package
	uploadInProgressPackage *Package
	LibArchive              libarchive.API
}

func NewServerBackend(la libarchive.API, dirpath string) (*ServerBackend, error) {
	sb := &ServerBackend{
		selectedPackage: nil, // this will be filled inside ProcessDirectory()
		LibArchive:      la,
	}

	err := sb.ProcessDirectory(dirpath)
	if err != nil {
		return nil, err
	}

	return sb, nil
}

func (sb *ServerBackend) parseUpdateMetadata(updateMetadataFilePath string) ([]byte, error) {
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

func (sb *ServerBackend) ProcessDirectory(dirpath string) error {
	var err error
	var packagesFound []string

	files, _ := ioutil.ReadDir(dirpath)
	for _, f := range files {
		if f.Name() == metadata.UpdateMetadataFilename {
			packagesFound = append(packagesFound, metadata.UpdateMetadataFilename)
		}

		if strings.HasSuffix(f.Name(), ".uhupkg") {
			packagesFound = append(packagesFound, f.Name())
		}
	}

	if len(packagesFound) == 0 {
		sb.selectedPackage = nil
		return nil
	}

	if len(packagesFound) > 1 {
		finalErr := fmt.Errorf("the path provided must not have more than 1 package. Found: %d", len(packagesFound))
		log.Error(finalErr)
		return finalErr
	}

	pkgpath := path.Join(dirpath, packagesFound[0])

	var updateMetadata []byte
	var signature []byte
	var isUhupkgSingleFile bool
	var finalPath string

	log.Info("selected package: ", pkgpath)
	if packagesFound[0] == metadata.UpdateMetadataFilename {
		isUhupkgSingleFile = false
		finalPath = dirpath
		updateMetadata, err = sb.parseUpdateMetadata(path.Join(dirpath, metadata.UpdateMetadataFilename))
	} else {
		isUhupkgSingleFile = true
		finalPath = pkgpath
		updateMetadata, signature, err = sb.parseUhuPkg(pkgpath)
	}

	if err != nil {
		return err
	}

	sb.selectedPackage = &Package{
		updateMetadata:     updateMetadata,
		signature:          signature,
		path:               finalPath,
		isUhupkgSingleFile: isUhupkgSingleFile,
	}

	log.Info("selected package-uid: ", utils.DataSha256sum(updateMetadata))
	log.Debug("update metadata loaded: \n", string(updateMetadata))

	return nil
}

func (sb *ServerBackend) Routes() []Route {
	routes := []Route{
		{Method: "POST", Path: "/upgrades", Handle: sb.getUpdateMetadata},
		{Method: "POST", Path: "/report", Handle: sb.reportStatus},
		{Method: "GET", Path: "/products/:product/packages/:package/objects/:object", Handle: sb.getObject},
		{Method: "POST", Path: "/packages", Handle: sb.receiveUpdateMetadata},
		{Method: "PUT", Path: "/packages/:package/finish", Handle: sb.finishUpload},
	}

	return routes
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

	if sb.selectedPackage.isUhupkgSingleFile {
		fileName := p.ByName("object")

		// package was already parsed, we can safely ignore the error here
		reader, _ := libarchive.NewReader(sb.LibArchive, sb.selectedPackage.path, 10240)
		defer reader.Free()

		err := reader.ExtractFile(fileName, w)
		if err != nil {
			log.Error(err)
			w.WriteHeader(500)
			fmt.Fprintf(w, "500 internal server error\n")
			return
		}
	} else {
		fileName := path.Join(sb.selectedPackage.path, p.ByName("product"), p.ByName("package"), p.ByName("object"))
		http.ServeFile(w, r, fileName)
	}
}

func (sb *ServerBackend) receiveUpdateMetadata(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	buffer := bytes.NewBuffer(nil)

	_, err := io.Copy(buffer, r.Body)
	if err != nil {
		log.Error(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "{ \"error_message\": \"%s\" }", err)
		return
	}

	tempPath, err := ioutil.TempDir("", "updatehub-server-pkg")
	if err != nil {
		log.Error(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "{ \"error_message\": \"%s\" }", err)
		return
	}

	updateMetadataFilePath := path.Join(tempPath, metadata.UpdateMetadataFilename)

	err = ioutil.WriteFile(updateMetadataFilePath, buffer.Bytes(), 0644)
	if err != nil {
		log.Error(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "{ \"error_message\": \"%s\" }", err)
		return
	}

	signature := []byte(r.Header.Get("UH-Signature"))
	sb.uploadInProgressPackage = &Package{
		updateMetadata:     buffer.Bytes(),
		signature:          signature,
		path:               tempPath,
		isUhupkgSingleFile: false,
	}

	packageUID := utils.DataSha256sum(buffer.Bytes())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	fmt.Fprintf(w, "{ \"uid\": \"%s\" }", packageUID)
}

func (sb *ServerBackend) finishUpload(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	packageUID := p.ByName("package")

	if sb.uploadInProgressPackage != nil &&
		sb.uploadInProgressPackage.updateMetadata != nil &&
		utils.DataSha256sum(sb.uploadInProgressPackage.updateMetadata) == packageUID {

		sb.selectedPackage = sb.uploadInProgressPackage
		sb.uploadInProgressPackage = nil

		w.WriteHeader(200)
		fmt.Fprintf(w, "{}")
		return
	}

	w.WriteHeader(404)
	fmt.Fprintf(w, "{ \"error_message\": \"Not found\" }")
}
