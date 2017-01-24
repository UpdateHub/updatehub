package client

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"bitbucket.org/ossystems/agent/metadata"
)

type UpdateClient struct {
}

type Updater interface {
	CheckUpdate(api ApiRequester) (interface{}, int, error)
}

func (u *UpdateClient) CheckUpdate(api ApiRequester) (interface{}, int, error) {
	url := serverURL(api.Client(), UpgradesEndpoint)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return nil, 0, errors.New("failed to create check update request")
	}

	res, err := api.Do(req)
	if err != nil {
		return nil, 0, errors.New("check update request failed")
	}

	defer res.Body.Close()

	var extraPoll int

	r, err := processUpgradeResponse(res)
	if err == nil {
		if v, ok := res.Header["Add-Extra-Poll"]; ok {
			extraPoll, err = strconv.Atoi(strings.Join(v, ""))
			if err != nil {
				return nil, 0, errors.New("failed to parse extra poll header")
			}
		}
	}

	return r, extraPoll, err
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
