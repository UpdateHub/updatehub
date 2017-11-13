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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/OSSystems/pkg/log"
	"github.com/anacrolix/missinggo/httptoo"
	"github.com/updatehub/updatehub/metadata"
)

type UpdateClient struct {
}

type Updater interface {
	ProbeUpdate(api ApiRequester, uri string, data interface{}) (interface{}, []byte, time.Duration, error)
	DownloadUpdate(api ApiRequester, uri string, cr *httptoo.BytesContentRange) (io.ReadCloser, int64, error)
	GetUpdateContentRange(api ApiRequester, uri string, start int64) (*httptoo.BytesContentRange, error)
}

func (u *UpdateClient) ProbeUpdate(api ApiRequester, uri string, data interface{}) (interface{}, []byte, time.Duration, error) {
	if api == nil {
		finalErr := fmt.Errorf("invalid api requester")
		log.Error(finalErr)
		return nil, nil, 0, finalErr
	}

	url := serverURL(api.Client(), uri)
	log.Debug("probing update at: ", url)

	rawJSON, _ := json.Marshal(data)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(rawJSON))
	if err != nil {
		finalErr := fmt.Errorf("failed to create probe update request: %s", err)
		log.Error(finalErr)
		return nil, nil, 0, finalErr
	}

	req.Header.Set("Api-Content-Type", "application/vnd.updatehub-v1+json")
	req.Header.Set("Content-Type", "application/json")

	res, err := api.Do(req)
	if err != nil {
		finalErr := fmt.Errorf("probe update request failed: %s", err)
		log.Error(finalErr)
		return nil, nil, 0, finalErr
	}

	defer res.Body.Close()

	var extraPoll int64

	r, signature, err := processUpgradeResponse(res)
	if err == nil {
		if v, ok := res.Header["Add-Extra-Poll"]; ok {
			extraPoll, err = strconv.ParseInt(strings.Join(v, ""), 10, 0)
			if err != nil {
				finalErr := fmt.Errorf("failed to parse extra poll header: %s", err)
				log.Error(finalErr)
				return nil, nil, 0, finalErr
			}
		}
	}

	return r, signature, time.Duration(extraPoll), err
}

func (u *UpdateClient) DownloadUpdate(api ApiRequester, uri string, cr *httptoo.BytesContentRange) (io.ReadCloser, int64, error) {
	if api == nil {
		finalErr := fmt.Errorf("invalid api requester")
		log.Error(finalErr)
		return nil, -1, finalErr
	}

	url := serverURL(api.Client(), uri)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		finalErr := fmt.Errorf("failed to create download update request: %s", err)
		log.Error(finalErr)
		return nil, -1, finalErr
	}

	req.Header.Set("Api-Content-Type", "application/vnd.updatehub-v1+json")

	if cr != nil {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", cr.First))
	}

	res, err := api.Do(req)
	if err != nil {
		finalErr := fmt.Errorf("download update request failed: %s", err)
		log.Error(finalErr)
		return nil, -1, finalErr
	}

	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		finalErr := fmt.Errorf("failed to download update. maybe the file is missing?")
		log.Error(finalErr)
		return nil, -1, finalErr
	}

	return res.Body, res.ContentLength, nil
}

func (u *UpdateClient) GetUpdateContentRange(api ApiRequester, uri string, start int64) (*httptoo.BytesContentRange, error) {
	if api == nil {
		finalErr := fmt.Errorf("invalid api requester")
		log.Error(finalErr)
		return nil, finalErr
	}

	url := serverURL(api.Client(), uri)

	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		finalErr := fmt.Errorf("failed to create update content range request: %s", err)
		log.Error(finalErr)
		return nil, finalErr
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))

	res, err := api.Do(req)
	if err != nil {
		finalErr := fmt.Errorf("get update content range failed: %s", err)
		log.Error(finalErr)
		return nil, finalErr
	}

	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		finalErr := fmt.Errorf("failed to get update content range. maybe the file is missing?")
		log.Error(finalErr)
		return nil, finalErr
	}

	cr, ok := httptoo.ParseBytesContentRange(res.Header.Get("Content-Range"))
	if !ok {
		finalErr := fmt.Errorf("error parsing content range")
		log.Error(finalErr)
		return nil, finalErr
	}

	return &cr, nil
}

func processUpgradeResponse(res *http.Response) (interface{}, []byte, error) {
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		finalErr := fmt.Errorf("error reading response body: %s", err)
		log.Error(finalErr)
		return nil, nil, finalErr
	}

	switch res.StatusCode {
	case http.StatusOK:
		data, err := metadata.NewUpdateMetadata(body)
		if err != nil {
			finalErr := fmt.Errorf("failed to parse upgrade response: %s", err)
			log.Error(finalErr)
			return nil, nil, finalErr
		}

		log.Info("Update available")

		signature, err := base64.StdEncoding.DecodeString(res.Header.Get("UH-Signature"))
		if err != nil {
			return data, nil, nil
		}

		return data, signature, nil
	case http.StatusNotFound:
		log.Info("Update not available")

		// NotFound is not an error in this case, just means there is no update available
		return nil, nil, nil
	}

	finalErr := fmt.Errorf("invalid response received from the server. HTTP code: %d", res.StatusCode)
	log.Error(finalErr)
	return nil, nil, finalErr
}

func NewUpdateClient() *UpdateClient {
	return &UpdateClient{}
}
