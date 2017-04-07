/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/bouk/monkey"
	"github.com/go-ini/ini"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"

	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
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
		extraPoll      time.Duration
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

func TestStartPolling(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name                 string
		pollingInterval      time.Duration
		extraPollingInterval time.Duration
		firstPoll            time.Time
		lastPoll             time.Time
		expectedState        State
		subTest              func(t *testing.T, uh *UpdateHub, state State)
	}{
		{
			"RegularPoll",
			time.Second,
			0,
			(time.Time{}).UTC(),
			(time.Time{}).UTC(),
			&PollState{},
			func(t *testing.T, uh *UpdateHub, state State) {},
		},

		{
			"NeverDidPollBefore",
			time.Second,
			0,
			now.Add(-1 * time.Second),
			(time.Time{}).UTC(),
			&UpdateCheckState{},
			func(t *testing.T, uh *UpdateHub, state State) {},
		},

		{
			"PendingRegularPoll",
			time.Second,
			0,
			now.Add(-4 * time.Second),
			now.Add(-2 * time.Second),
			&UpdateCheckState{},
			func(t *testing.T, uh *UpdateHub, state State) {},
		},

		{
			"PendingExtraPoll",
			10 * time.Second,
			3 * time.Second,
			now.Add(-25 * time.Second),
			now.Add(-5 * time.Second),
			&PollState{},
			func(t *testing.T, uh *UpdateHub, state State) {
				poll := state.(*PollState)
				assert.Equal(t, 3*time.Second, poll.interval)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate time passage from now
			defer func() *monkey.PatchGuard {
				seconds := -1
				return monkey.Patch(time.Now, func() time.Time {
					seconds++
					return now.Add(time.Second * time.Duration(seconds))
				})
			}().Unpatch()

			uh, _ := newTestUpdateHub(nil)

			uh.settings.PollingInterval = tc.pollingInterval
			uh.settings.ExtraPollingInterval = tc.extraPollingInterval
			uh.settings.FirstPoll = tc.firstPoll
			uh.settings.LastPoll = tc.lastPoll

			uh.StartPolling()
			assert.IsType(t, tc.expectedState, uh.state)

			tc.subTest(t, uh, uh.state)
		})
	}
}

func TestLoadUpdateHubSettings(t *testing.T) {
	testCases := []struct {
		name            string
		systemSettings  string
		runtimeSettings string
		expectedError   interface{}
		subTest         func(t *testing.T, settings *Settings, err error)
	}{
		{
			"SystemSettingsNotFound",
			"",
			"",
			&os.PathError{},
			func(t *testing.T, settings *Settings, err error) {
				assert.Equal(t, err.(*os.PathError).Path, systemSettingsPath)
			},
		},

		{
			"RuntimeSettingsNotFound",
			"[Polling]\nEnabled=true",
			"",
			&os.PathError{},
			func(t *testing.T, settings *Settings, err error) {
				assert.Equal(t, err.(*os.PathError).Path, runtimeSettingsPath)
			},
		},

		{
			"InvalidSettingsFile",
			"test",
			"test",
			ini.ErrDelimiterNotFound{},
			func(t *testing.T, settings *Settings, err error) {
			},
		},

		{
			"ValidSettingsFile",
			"[Polling]\nEnabled=true",
			"[Polling]\nExtraInterval=1",
			nil,
			func(t *testing.T, settings *Settings, err error) {
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uh, _ := newTestUpdateHub(nil)

			if tc.systemSettings != "" {
				err := uh.store.MkdirAll(filepath.Dir(systemSettingsPath), 0755)
				assert.NoError(t, err)
				err = afero.WriteFile(uh.store, systemSettingsPath, []byte(tc.systemSettings), 0644)
				assert.NoError(t, err)
			}

			if tc.runtimeSettings != "" {
				err := uh.store.MkdirAll(filepath.Dir(runtimeSettingsPath), 0755)
				assert.NoError(t, err)
				err = afero.WriteFile(uh.store, runtimeSettingsPath, []byte(tc.runtimeSettings), 0644)
				assert.NoError(t, err)
			}

			err := uh.LoadSettings()
			assert.IsType(t, tc.expectedError, err)

			tc.subTest(t, uh.settings, err)
		})
	}
}

type testObject struct {
	metadata.ObjectMetadata
}

type testUpdater struct {
	// CheckUpdate
	updateMetadata   *metadata.UpdateMetadata
	extraPoll        time.Duration
	checkUpdateError error
	// FetchUpdate
	updateBytes      []byte
	fetchUpdateError error
}

func (t testUpdater) CheckUpdate(api client.ApiRequester, data interface{}) (interface{}, time.Duration, error) {
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
		store:    afero.NewMemMapFs(),
		state:    state,
		timeStep: time.Second,
		api:      client.NewApiClient("localhost"),
	}

	settings, err := LoadSettings(bytes.NewReader([]byte("")))
	if err != nil {
		return nil, err
	}

	uh.settings = settings
	uh.settings.PollingInterval = 1

	return uh, err
}
