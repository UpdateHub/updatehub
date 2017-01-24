package client

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"bitbucket.org/ossystems/agent/metadata"
)

type UpdateClient struct {
}

type Updater interface {
	CheckUpdate(api ApiRequester) (interface{}, error)
}

func (u *UpdateClient) CheckUpdate(api ApiRequester) (interface{}, error) {
	url := serverURL(api.Client(), UpgradesEndpoint)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return nil, errors.New("failed to create check update request")
	}

	res, err := api.Do(req)
	if err != nil {
		return nil, errors.New("check update request failed")
	}

	defer res.Body.Close()

	r, err := processUpgradeResponse(res)

	return r, err
}

func processUpgradeResponse(res *http.Response) (interface{}, error) {
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	switch res.StatusCode {
	case http.StatusOK:
		var data metadata.Metadata

		if err := json.Unmarshal(body, &data); err != nil {
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
