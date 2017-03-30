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
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/UpdateHub/updatehub/metadata"
)

type UpdateClient struct {
}

type Updater interface {
	CheckUpdate(api ApiRequester, data interface{}) (interface{}, time.Duration, error)
	FetchUpdate(api ApiRequester, uri string) (io.ReadCloser, int64, error)
}

func (u *UpdateClient) CheckUpdate(api ApiRequester, data interface{}) (interface{}, time.Duration, error) {
	if api == nil {
		return nil, 0, errors.New("invalid api requester")
	}

	rawJSON, _ := json.Marshal(data)

	url := serverURL(api.Client(), UpgradesEndpoint)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(rawJSON))
	if err != nil {
		return nil, 0, errors.New("failed to create check update request")
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := api.Do(req)
	if err != nil {
		return nil, 0, errors.New("check update request failed")
	}

	defer res.Body.Close()

	var extraPoll int64

	r, err := processUpgradeResponse(res)
	if err == nil {
		if v, ok := res.Header["Add-Extra-Poll"]; ok {
			extraPoll, err = strconv.ParseInt(strings.Join(v, ""), 10, 0)
			if err != nil {
				return nil, 0, errors.New("failed to parse extra poll header")
			}
		}
	}

	return r, time.Duration(extraPoll), err
}

func (u *UpdateClient) FetchUpdate(api ApiRequester, uri string) (io.ReadCloser, int64, error) {
	if api == nil {
		return nil, -1, errors.New("invalid api requester")
	}

	url := serverURL(api.Client(), uri)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, -1, errors.New("failed to create fetch update request")
	}

	res, err := api.Do(req)
	if err != nil {
		return nil, -1, errors.New("fetch update request failed")
	}

	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, -1, errors.New("failed to fetch update. maybe the file is missing?")
	}

	return res.Body, res.ContentLength, nil
}

func processUpgradeResponse(res *http.Response) (interface{}, error) {
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	switch res.StatusCode {
	case http.StatusOK:
		data, err := metadata.NewUpdateMetadata(body)
		if err != nil {
			return nil, errors.New("failed to parse upgrade response")
		}

		return data, nil
	case http.StatusNotFound:
		// NotFound is not an error in this case, just means there is no update available
		return nil, nil
	}

	return nil, errors.New("invalid response received from the server")
}

func NewUpdateClient() *UpdateClient {
	return &UpdateClient{}
}
