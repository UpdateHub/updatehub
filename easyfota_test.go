package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"bitbucket.org/ossystems/agent/client"
	"bitbucket.org/ossystems/agent/installmodes"
	"bitbucket.org/ossystems/agent/metadata"
)

const validUpdateMetadata = `{
  "product-uid": "123",
  "objects": [
    [
      { "mode": "test", "sha256sum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" }
    ]
  ]
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
			expectedUpdateMetadata, _ := metadata.NewUpdateMetadata([]byte(tc.updateMetadata))

			updater := testUpdater{
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

func TestEasyFotaFetchUpdate(t *testing.T) {
	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return &testObject{} },
	})

	defer mode.Unregister()

	fota := &EasyFota{
		state:    &PollState{},
		timeStep: time.Millisecond,
		api:      client.NewApiClient("localhost"),
	}

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	updater := testUpdater{
		updateMetadata: updateMetadata,
		extraPoll:      0,
		updateBytes:    []byte("0123456789"),
	}

	fota.updater = client.Updater(updater)

	err = fota.FetchUpdate(updateMetadata, nil)
	assert.NoError(t, err)

	data, err := ioutil.ReadFile(path.Join("/tmp", updateMetadata.Objects[0][0].GetObjectMetadata().Sha256sum))
	assert.NoError(t, err)
	assert.Equal(t, updater.updateBytes, data)
}

type testObject struct {
	metadata.ObjectMetadata
}

type testUpdater struct {
	// CheckUpdate
	updateMetadata   *metadata.UpdateMetadata
	extraPoll        int
	checkUpdateError error
	// FetchUpdate
	updateBytes      []byte
	fetchUpdateError error
}

func (t testUpdater) CheckUpdate(api client.ApiRequester) (interface{}, int, error) {
	return t.updateMetadata, t.extraPoll, t.checkUpdateError
}

func (t testUpdater) FetchUpdate(api client.ApiRequester, uri string) (io.ReadCloser, int64, error) {
	rd := bytes.NewReader(t.updateBytes)
	return ioutil.NopCloser(rd), int64(len(t.updateBytes)), t.fetchUpdateError
}
