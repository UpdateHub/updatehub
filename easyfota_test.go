package main

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"bitbucket.org/ossystems/agent/client"
	"bitbucket.org/ossystems/agent/metadata"
)

type UpdaterTest struct {
	updateMetadata   *metadata.UpdateMetadata
	extraPoll        int
	checkUpdateError error
}

func (u UpdaterTest) CheckUpdate(api client.ApiRequester) (interface{}, int, error) {
	return u.updateMetadata, u.extraPoll, u.checkUpdateError
}

func (u UpdaterTest) FetchUpdate(api client.ApiRequester, uri string) (io.ReadCloser, int64, error) {
	return nil, 0, nil
}

const validUpdateMetadata = `{
  "product-uid": "123",
  "objects": [ [ ] ]
}`

var checkUpdateTestCases = []struct {
	name           string
	updateMetadata string
	extraPoll      int
	err            error
}{
	{
		"InvalidUpdateMetadata",
		"",
		0,
		nil,
	},

	{
		"ValidUpdateMetadata",
		validUpdateMetadata,
		13,
		nil,
	},
}

func TestEasyfotaCheckUpdate(t *testing.T) {
	fota := &EasyFota{
		state:    &PollState{},
		timeStep: time.Millisecond,
		api:      client.NewApiClient("localhost"),
	}

	for _, tc := range checkUpdateTestCases {
		t.Run(tc.name, func(t *testing.T) {
			expectedUpdateMetadata, _ := metadata.FromJSON([]byte(tc.updateMetadata))

			updater := UpdaterTest{
				updateMetadata: expectedUpdateMetadata,
				extraPoll:      tc.extraPoll,
			}

			fota.updater = client.Updater(updater)

			updateMetadata, extraPoll := fota.CheckUpdate()

			assert.Equal(t, expectedUpdateMetadata, updateMetadata)
			assert.Equal(t, tc.extraPoll, extraPoll)
		})
	}
}
