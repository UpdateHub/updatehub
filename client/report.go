/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/OSSystems/pkg/log"
	"github.com/UpdateHub/updatehub/metadata"
)

type ReportClient struct {
}

type Reporter interface {
	ReportState(api ApiRequester, packageUID string, previousState string, state string, errorMessage string, fm metadata.FirmwareMetadata) error
}

func (u *ReportClient) ReportState(api ApiRequester, packageUID string, previousState string, state string, errorMessage string, fm metadata.FirmwareMetadata) error {
	log.Debug("reporting state: ", state)
	log.Debug("  error message: ", errorMessage)
	log.Debug("  packageUID: ", packageUID)
	log.Debug("  previous state: ", previousState)

	if api == nil {
		finalErr := fmt.Errorf("invalid api requester")
		log.Error(finalErr)
		return finalErr
	}

	url := serverURL(api.Client(), StateReportEndpoint)

	data := make(map[string]interface{})
	data["status"] = state
	data["previous-state"] = previousState
	data["package-uid"] = packageUID
	data["error-message"] = errorMessage
	data["device-attributes"] = fm.DeviceAttributes
	data["product-uid"] = fm.ProductUID
	data["device-identity"] = fm.DeviceIdentity
	data["version"] = fm.Version
	data["hardware"] = fm.Hardware

	body, err := json.Marshal(data)
	if err != nil {
		finalErr := fmt.Errorf("failed to marshal request body: %s", err)
		log.Error(finalErr)
		return finalErr
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		finalErr := fmt.Errorf("failed to create report request: %s", err)
		log.Error(finalErr)
		return finalErr
	}

	req.Header.Set("Api-Content-Type", "application/vnd.updatehub-v1+json")
	req.Header.Set("Content-Type", "application/json")

	res, err := api.Do(req)
	if err != nil {
		finalErr := fmt.Errorf("report request failed: %s", err)
		log.Error(finalErr)
		return finalErr
	}

	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		log.Info("state '", state, "' reported successfully")
		return nil
	}

	responseBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		finalErr := fmt.Errorf("failed to read report response body: %s", err)
		log.Error(finalErr)
		return finalErr
	}

	log.Debug("report response body content: ", string(responseBody))

	finalErr := fmt.Errorf("failed to report state '%s'. HTTP code: %d", state, res.StatusCode)
	log.Error(finalErr)
	return finalErr
}

func NewReportClient() *ReportClient {
	return &ReportClient{}
}
