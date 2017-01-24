package client

import (
	"fmt"
	"net/http"
)

// TODO: https support

const (
	UpgradesEndpoint    = "/upgrades"
	StateReportEndpoint = "/report"
)

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
	return &ApiClient{Client: http.Client{}, server: server}
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
	return fmt.Sprintf("http://%s/%s", c.server, path[1:])
}
