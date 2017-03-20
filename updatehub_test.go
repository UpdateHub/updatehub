package main

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"path"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"

	"code.ossystems.com.br/updatehub/agent/client"
	"code.ossystems.com.br/updatehub/agent/installmodes"
	"code.ossystems.com.br/updatehub/agent/metadata"
)

const validUpdateMetadata = `{
  "product-uid": "123",
  "objects": [
    [
      { "mode": "test", "sha256sum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" }
    ]
  ]
}`

func TestUpdateHubCheckUpdate(t *testing.T) {
	testCases := []struct {
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

	uh, _ := newTestUpdateHub(&PollState{})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expectedUpdateMetadata, _ := metadata.NewUpdateMetadata([]byte(tc.updateMetadata))

			updater := testUpdater{
				updateMetadata: expectedUpdateMetadata,
				extraPoll:      tc.extraPoll,
			}

			uh.updater = client.Updater(updater)

			updateMetadata, extraPoll := uh.CheckUpdate(0)

			assert.Equal(t, expectedUpdateMetadata, updateMetadata)
			assert.Equal(t, tc.extraPoll, extraPoll)
		})
	}
}

func TestUpdateHubFetchUpdate(t *testing.T) {
	mode := newTestInstallMode()

	defer mode.Unregister()

	uh, _ := newTestUpdateHub(&PollState{})

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	updater := testUpdater{
		updateMetadata: updateMetadata,
		extraPoll:      0,
		updateBytes:    []byte("0123456789"),
	}

	uh.updater = client.Updater(updater)

	err = uh.FetchUpdate(updateMetadata, nil)
	assert.NoError(t, err)

	data, err := afero.ReadFile(uh.store, path.Join(uh.settings.DownloadDir, updateMetadata.Objects[0][0].GetObjectMetadata().Sha256sum))
	assert.NoError(t, err)
	assert.Equal(t, updater.updateBytes, data)
}

func TestUpdateHubReportState(t *testing.T) {
	mode := newTestInstallMode()

	defer mode.Unregister()

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	state := &testReportableState{}
	state.updateMetadata = updateMetadata

	uh, _ := newTestUpdateHub(state)
	uh.reporter = client.Reporter(testReporter{})

	err = uh.ReportCurrentState()
	assert.NoError(t, err)

	uh.reporter = client.Reporter(testReporter{reportStateError: errors.New("error")})

	err = uh.ReportCurrentState()
	assert.Error(t, err)
	assert.EqualError(t, err, "error")
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

func (t testUpdater) CheckUpdate(api client.ApiRequester, data interface{}) (interface{}, int, error) {
	return t.updateMetadata, t.extraPoll, t.checkUpdateError
}

func (t testUpdater) FetchUpdate(api client.ApiRequester, uri string) (io.ReadCloser, int64, error) {
	rd := bytes.NewReader(t.updateBytes)
	return ioutil.NopCloser(rd), int64(len(t.updateBytes)), t.fetchUpdateError
}

type testReporter struct {
	reportStateError error
}

func (r testReporter) ReportState(api client.ApiRequester, packageUID string, state string) error {
	return r.reportStateError
}

func newTestInstallMode() installmodes.InstallMode {
	return installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return &testObject{} },
	})
}

func newTestUpdateHub(state State) (*UpdateHub, error) {
	uh := &UpdateHub{
		store:        afero.NewMemMapFs(),
		state:        state,
		timeStep:     time.Millisecond,
		pollInterval: 1,
		api:          client.NewApiClient("localhost"),
	}

	settings, err := LoadSettings(bytes.NewReader([]byte("")))
	if err != nil {
		return nil, err
	}

	uh.settings = settings

	return uh, err
}
