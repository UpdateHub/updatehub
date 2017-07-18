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
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/OSSystems/pkg/log"
	"github.com/UpdateHub/updatehub/metadata"
)

type UpdateClient struct {
}

type Updater interface {
	CheckUpdate(api ApiRequester, uri string, data interface{}) (interface{}, time.Duration, error)
	FetchUpdate(api ApiRequester, uri string) (io.ReadCloser, int64, error)
}

func (u *UpdateClient) CheckUpdate(api ApiRequester, uri string, data interface{}) (interface{}, time.Duration, error) {
	if api == nil {
		finalErr := fmt.Errorf("invalid api requester")
		log.Error(finalErr)
		return nil, 0, finalErr
	}

	url := serverURL(api.Client(), uri)
	log.Debug("checking update at: ", url)

	rawJSON, _ := json.Marshal(data)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(rawJSON))
	if err != nil {
		finalErr := fmt.Errorf("failed to create check update request: %s", err)
		log.Error(finalErr)
		return nil, 0, finalErr
	}

	req.Header.Set("Api-Content-Type", "application/vnd.updatehub-v1+json")
	req.Header.Set("Content-Type", "application/json")

	res, err := api.Do(req)
	if err != nil {
		finalErr := fmt.Errorf("check update request failed: %s", err)
		log.Error(finalErr)
		return nil, 0, finalErr
	}

	defer res.Body.Close()

	var extraPoll int64

	r, err := processUpgradeResponse(res)
	if err == nil {
		if v, ok := res.Header["Add-Extra-Poll"]; ok {
			extraPoll, err = strconv.ParseInt(strings.Join(v, ""), 10, 0)
			if err != nil {
				finalErr := fmt.Errorf("failed to parse extra poll header: %s", err)
				log.Error(finalErr)
				return nil, 0, finalErr
			}
		}
	}

	return r, time.Duration(extraPoll), err
}

func (u *UpdateClient) FetchUpdate(api ApiRequester, uri string) (io.ReadCloser, int64, error) {
	if api == nil {
		finalErr := fmt.Errorf("invalid api requester")
		log.Error(finalErr)
		return nil, -1, finalErr
	}

	url := serverURL(api.Client(), uri)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		finalErr := fmt.Errorf("failed to create fetch update request: %s", err)
		log.Error(finalErr)
		return nil, -1, finalErr
	}

	req.Header.Set("Api-Content-Type", "application/vnd.updatehub-v1+json")

	res, err := api.Do(req)
	if err != nil {
		finalErr := fmt.Errorf("fetch update request failed: %s", err)
		log.Error(finalErr)
		return nil, -1, finalErr
	}

	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		finalErr := fmt.Errorf("failed to fetch update. maybe the file is missing?")
		log.Error(finalErr)
		return nil, -1, finalErr
	}

	return res.Body, res.ContentLength, nil
}

func processUpgradeResponse(res *http.Response) (interface{}, error) {
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		finalErr := fmt.Errorf("error reading response body: %s", err)
		log.Error(finalErr)
		return nil, finalErr
	}

	switch res.StatusCode {
	case http.StatusOK:
		data, err := metadata.NewUpdateMetadata(body)
		if err != nil {
			finalErr := fmt.Errorf("failed to parse upgrade response: %s", err)
			log.Error(finalErr)
			return nil, finalErr
		}

		return data, nil
	case http.StatusNotFound:
		// NotFound is not an error in this case, just means there is no update available
		return nil, nil
	}

	finalErr := fmt.Errorf("invalid response received from the server. HTTP code: %d", res.StatusCode)
	log.Error(finalErr)
	return nil, finalErr
}

func NewUpdateClient() *UpdateClient {
	return &UpdateClient{}
}
