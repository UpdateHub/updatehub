/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package client

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// TODO: https support

const (
	UpgradesEndpoint    = "/upgrades"
	StateReportEndpoint = "/report"
)

const defaultRedirectLimit = 10

var ErrMaxRedirect = errors.New("Exceeded max redirects")

var defaultHttpTransport = http.Transport{
	Dial: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).Dial,
	TLSHandshakeTimeout:   30 * time.Second,
	ResponseHeaderTimeout: 30 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

type ApiClient struct {
	http.Client

	server string
}

func (client *ApiClient) Request() *ApiRequest {
	return &ApiRequest{
		client: client,
	}
}

func NewApiClient(server string) *ApiClient {
	return &ApiClient{
		Client: http.Client{
			Transport:     &defaultHttpTransport,
			CheckRedirect: checkRedirect,
		},
		server: server,
	}
}

type ApiRequest struct {
	client *ApiClient
}

type ApiRequester interface {
	Client() *ApiClient
	Do(req *http.Request) (*http.Response, error)
}

func (r *ApiRequest) Client() *ApiClient {
	return r.client
}

func (r *ApiRequest) Do(req *http.Request) (*http.Response, error) {
	return r.client.Do(req)
}

func serverURL(c *ApiClient, path string) string {
	return fmt.Sprintf("%s/%s", c.server, path[1:])
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > defaultRedirectLimit {
		return ErrMaxRedirect
	}

	if len(via) == 0 {
		return nil
	}

	for key, val := range via[0].Header {
		if key != "Referer" {
			req.Header[key] = val
		}
	}

	return nil
}
