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
	"net/http"

	"github.com/OSSystems/pkg/log"
)

type ReportClient struct {
}

type Reporter interface {
	ReportState(api ApiRequester, packageUID string, state string) error
}

func (u *ReportClient) ReportState(api ApiRequester, packageUID string, state string) error {
	log.Info("Reporting state: ", state)
	log.Info("PackageUID: ", packageUID)

	if api == nil {
		finalErr := fmt.Errorf("invalid api requester")
		log.Error(finalErr)
		return finalErr
	}

	url := serverURL(api.Client(), StateReportEndpoint)

	data := make(map[string]interface{})
	data["status"] = state
	data["package-uid"] = packageUID
	data["error-message"] = ""

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

	res, err := api.Do(req)
	if err != nil {
		finalErr := fmt.Errorf("report request failed: %s", err)
		log.Error(finalErr)
		return finalErr
	}

	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		log.Info("State ", state, " reported successfully")
		return nil
	}

	finalErr := fmt.Errorf("failed to report state. HTTP code: %d", res.StatusCode)
	log.Error(finalErr)
	return finalErr
}

func NewReportClient() *ReportClient {
	return &ReportClient{}
}
