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
	"errors"
	"net/http"
)

type ReportClient struct {
}

type Reporter interface {
	ReportState(api ApiRequester, packageUID string, state string) error
}

func (u *ReportClient) ReportState(api ApiRequester, packageUID string, state string) error {
	if api == nil {
		return errors.New("invalid api requester")
	}

	url := serverURL(api.Client(), StateReportEndpoint)

	data := make(map[string]interface{})
	data["status"] = state
	data["package-uid"] = packageUID
	data["error-message"] = ""

	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return errors.New("failed to create report request")
	}

	res, err := api.Do(req)
	if err != nil {
		return errors.New("report request failed")
	}

	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		return nil
	}

	return errors.New("failed to report state")
}

func NewReportClient() *ReportClient {
	return &ReportClient{}
}
