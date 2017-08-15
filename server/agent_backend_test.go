/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/cmdlinemock"
	"github.com/UpdateHub/updatehub/testsmocks/controllermock"
	"github.com/UpdateHub/updatehub/testsmocks/rebootermock"
	"github.com/UpdateHub/updatehub/testsmocks/reportermock"
	"github.com/UpdateHub/updatehub/testsmocks/responsewritermock"
	"github.com/UpdateHub/updatehub/updatehub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	customSettings = `
[Polling]
Interval=1d
Enabled=false
LastPoll=2017-01-01T00:00:00Z
FirstPoll=2017-02-02T00:00:00Z
ExtraInterval=4
Retries=5

[Storage]
ReadOnly=true

[Update]
DownloadDir=/tmp/download
SupportedInstallModes=mode1,mode2

[Network]
ServerAddress=http://localhost

[Firmware]
MetadataPath=/tmp/metadata
`
	validUpdateMetadataWithActiveInactive = `{
  "product-uid": "123",
  "objects": [
    [
      { "mode": "test", "target": "/dev/xxa1", "sha256sum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" },
      { "mode": "test", "target": "/dev/xxa2", "sha256sum": "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb" }
    ]
    ,
    [
      { "mode": "test", "target": "/dev/xxb1", "sha256sum": "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa" },
      { "mode": "test", "target": "/dev/xxb2", "sha256sum": "b9632efa90820ff35d6cec0946f99bb8a6317b1e2ef877e501a3e12b2d04d0ae" }
    ]
  ]
}`
)

func TestNewAgentBackend(t *testing.T) {
	uh := &updatehub.UpdateHub{}

	s := updatehub.DefaultSettings
	uh.Settings = &s

	ab, err := NewAgentBackend(uh)
	assert.NoError(t, err)

	routes := ab.Routes()
	assert.Equal(t, 4, len(routes))

	assert.Equal(t, "GET", routes[0].Method)
	assert.Equal(t, "/info", routes[0].Path)
	expectedFunction := reflect.ValueOf(ab.info)
	receivedFunction := reflect.ValueOf(routes[0].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())

	assert.Equal(t, "GET", routes[1].Method)
	assert.Equal(t, "/log", routes[1].Path)
	expectedFunction = reflect.ValueOf(ab.log)
	receivedFunction = reflect.ValueOf(routes[1].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())

	assert.Equal(t, "POST", routes[2].Method)
	assert.Equal(t, "/probe", routes[2].Path)
	expectedFunction = reflect.ValueOf(ab.probe)
	receivedFunction = reflect.ValueOf(routes[2].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())

	assert.Equal(t, "POST", routes[3].Method)
	assert.Equal(t, "/update/download/abort", routes[3].Path)
	expectedFunction = reflect.ValueOf(ab.updateDownloadAbort)
	receivedFunction = reflect.ValueOf(routes[3].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())
}

func setup(t *testing.T) (*updatehub.UpdateHub, *AgentBackend, *cmdlinemock.CmdLineExecuterMock, *rebootermock.RebooterMock) {
	const (
		metadataPath       = "/tmp/metadata"
		systemSettingsPath = "/system.conf"
	)

	// setup mem map filesystem
	fs := afero.NewMemMapFs()

	files := map[string]string{
		"/tmp/metadata/device-identity.d/key1":    "id1=value1",
		"/tmp/metadata/device-identity.d/key2":    "id2=value2",
		"/tmp/metadata/device-attributes.d/attr1": "attr1=value",
		"/tmp/metadata/device-attributes.d/attr2": "attr2=value",
	}

	for k, v := range files {
		err := afero.WriteFile(fs, k, []byte(v), 0700)
		assert.NoError(t, err)
	}

	err := afero.WriteFile(fs, systemSettingsPath, []byte(customSettings), 0644)
	assert.NoError(t, err)

	// setup mock
	productUID := "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381"
	hardware := "board"
	firmwareVersion := "1.1"
	agentVersion := "0.6.90-7-ga456673"
	buildTime := "2017-06-01 17:24 UTC"

	clm := &cmdlinemock.CmdLineExecuterMock{}

	clm.On("Execute", path.Join(metadataPath, "product-uid")).Return([]byte(productUID), nil).Once()
	clm.On("Execute", path.Join(metadataPath, "hardware")).Return([]byte(hardware), nil).Once()
	clm.On("Execute", path.Join(metadataPath, "version")).Return([]byte(firmwareVersion), nil).Once()
	clm.On("Execute", path.Join(metadataPath, "/device-identity.d/key1")).Return([]byte("id1=value1"), nil).Once()
	clm.On("Execute", path.Join(metadataPath, "/device-identity.d/key2")).Return([]byte("id2=value2"), nil).Once()
	clm.On("Execute", path.Join(metadataPath, "/device-attributes.d/attr1")).Return([]byte("attr1=value1"), nil).Once()
	clm.On("Execute", path.Join(metadataPath, "/device-attributes.d/attr2")).Return([]byte("attr2=value2"), nil).Once()

	// create objects
	file, err := fs.Open(systemSettingsPath)
	assert.NoError(t, err)
	defer file.Close()

	settings, err := updatehub.LoadSettings(file)
	assert.NoError(t, err)

	fm, err := metadata.NewFirmwareMetadata(metadataPath, fs, clm)
	assert.NoError(t, err)
	assert.NotNil(t, fm)

	rm := &rebootermock.RebooterMock{}

	uh := &updatehub.UpdateHub{
		FirmwareMetadata: *fm,
		Settings:         settings,
		Store:            fs,
		Version:          agentVersion,
		BuildTime:        buildTime,
		Rebooter:         rm,
		CmdLineExecuter:  clm,
	}

	ab, err := NewAgentBackend(uh)
	assert.NoError(t, err)

	return uh, ab, clm, rm
}

func teardown(t *testing.T) {
}

func TestInfoRoute(t *testing.T) {
	uh, ab, clm, rm := setup(t)
	defer teardown(t)

	jsonMap := map[string]interface{}{}

	jsonMap["version"] = uh.Version
	jsonMap["build-time"] = uh.BuildTime
	jsonMap["config"] = uh.Settings
	jsonMap["firmware"] = uh.FirmwareMetadata

	expectedJSON, err := json.MarshalIndent(jsonMap, "", "    ")
	assert.NoError(t, err)

	rwm := &responsewritermock.ResponseWriterMock{}
	rwm.On("WriteHeader", 200).Return()
	rwm.On("Write", expectedJSON).Return(len(expectedJSON), nil)

	ab.info(rwm, nil, nil)

	clm.AssertExpectations(t)
	rm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}

func TestProbeRouteWithDefaultApiClient(t *testing.T) {
	out := map[string]interface{}{}
	out["update-available"] = false
	out["try-again-in"] = 3600

	expectedResponse, _ := json.MarshalIndent(out, "", "    ")

	uh, ab, clm, rm := setup(t)
	defer teardown(t)

	uh.TimeStep = time.Second
	s := updatehub.NewIdleState()
	uh.SetState(s)

	cm := &controllermock.ControllerMock{}

	uh.Controller = cm
	cm.On("ProbeUpdate", uh.DefaultApiClient, 5).Return((*metadata.UpdateMetadata)(nil), 3600*time.Second)

	rwm := &responsewritermock.ResponseWriterMock{}
	rwm.On("WriteHeader", 200)
	rwm.On("Write", expectedResponse).Return(len(expectedResponse), nil)

	go func() {
		ok := false
		for ok == false {
			_, ok = s.NextState().(*updatehub.ProbeState)
			time.Sleep(100 * time.Millisecond)
		}

		s.NextState().Handle(uh)
	}()

	ab.probe(rwm, nil, nil)

	assert.IsType(t, &updatehub.ProbeState{}, s.NextState())

	clm.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}

func TestProbeRouteWithServerAddressField(t *testing.T) {
	testCases := []struct {
		Name        string
		Address     string
		ExpectedURL string
	}{
		{
			"ServerAddressAlreadySanitized",
			"http://different-address:8080",
			"http://different-address:8080",
		},
		{
			"ServerAddressNonSanitized",
			"different-address:8080",
			"https://different-address:8080",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			out := map[string]interface{}{}
			out["update-available"] = false
			out["try-again-in"] = 3600

			expectedResponse, _ := json.MarshalIndent(out, "", "    ")

			uh, ab, clm, rm := setup(t)
			defer teardown(t)

			apiClient := client.NewApiClient(tc.ExpectedURL)

			uh.TimeStep = time.Second
			s := updatehub.NewIdleState()
			uh.SetState(s)

			cm := &controllermock.ControllerMock{}

			uh.Controller = cm
			cm.On("ProbeUpdate", apiClient, 5).Return((*metadata.UpdateMetadata)(nil), 3600*time.Second)

			rwm := &responsewritermock.ResponseWriterMock{}
			rwm.On("WriteHeader", 200)
			rwm.On("Write", expectedResponse).Return(len(expectedResponse), nil)

			go func() {
				ok := false
				for ok == false {
					_, ok = s.NextState().(*updatehub.ProbeState)
					time.Sleep(100 * time.Millisecond)
				}

				s.NextState().Handle(uh)
			}()

			body := bytes.NewBufferString(fmt.Sprintf(`{ "server-address": "%s" }`, tc.ExpectedURL))

			req, err := http.NewRequest("POST", tc.Address, body)
			assert.NoError(t, err)

			ab.probe(rwm, req, nil)

			assert.IsType(t, &updatehub.ProbeState{}, s.NextState())

			clm.AssertExpectations(t)
			cm.AssertExpectations(t)
			rm.AssertExpectations(t)
			rwm.AssertExpectations(t)
		})
	}
}

func TestUpdateDownloadAbortRoute(t *testing.T) {
	expectedJSON := []byte(`{ "message": "request accepted, download aborted" }`)

	uh, ab, clm, rm := setup(t)
	defer teardown(t)

	cm := &controllermock.ControllerMock{}
	uh.Controller = cm

	repm := &reportermock.ReporterMock{}
	uh.Reporter = repm

	uh.TimeStep = time.Second

	ds := updatehub.NewDownloadingState(uh.DefaultApiClient, nil, &updatehub.ProgressTrackerImpl{})

	uh.SetState(ds)

	rwm := &responsewritermock.ResponseWriterMock{}
	rwm.On("WriteHeader", 200)
	rwm.On("Write", expectedJSON).Return(len(expectedJSON), nil)

	ab.updateDownloadAbort(rwm, nil, nil)

	assert.IsType(t, &updatehub.IdleState{}, ds.NextState())

	clm.AssertExpectations(t)
	cm.AssertExpectations(t)
	repm.AssertExpectations(t)
	rm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}

func TestUpdateDownloadAbortRouteWithNoDownloadInProgress(t *testing.T) {
	out := map[string]interface{}{}
	out["error"] = "there is no download to be aborted"

	expectedResponse, _ := json.MarshalIndent(out, "", "    ")

	uh, ab, clm, rm := setup(t)
	defer teardown(t)

	cm := &controllermock.ControllerMock{}
	uh.Controller = cm

	repm := &reportermock.ReporterMock{}
	uh.Reporter = repm

	uh.TimeStep = time.Second
	uh.SetState(updatehub.NewIdleState())

	rwm := &responsewritermock.ResponseWriterMock{}
	rwm.On("WriteHeader", 400)
	rwm.On("Write", expectedResponse).Return(len(expectedResponse), nil)

	ab.updateDownloadAbort(rwm, nil, nil)

	assert.IsType(t, &updatehub.ErrorState{}, uh.GetState())

	clm.AssertExpectations(t)
	cm.AssertExpectations(t)
	repm.AssertExpectations(t)
	rm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}

func TestLogRoute(t *testing.T) {
	_, ab, clm, rm := setup(t)
	defer teardown(t)

	logContent := []uint8("")

	rwm := &responsewritermock.ResponseWriterMock{}
	rwm.On("WriteHeader", 200)
	rwm.On("Write", mock.Anything).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		logContent = arg
	}).Return(len(logContent), nil)

	ab.log(rwm, nil, nil)

	jsonArray := []map[string]interface{}{}
	err := json.Unmarshal(logContent, &jsonArray)
	assert.NoError(t, err)

	assert.True(t, len(jsonArray) > 0)

	// since we can't control which log messages will be on the
	// buffer, we only test whether the field exists or not

	_, ok := jsonArray[0]["level"]
	assert.True(t, ok)

	_, ok = jsonArray[0]["message"]
	assert.True(t, ok)

	_, ok = jsonArray[0]["time"]
	assert.True(t, ok)

	_, ok = jsonArray[0]["data"]
	assert.True(t, ok)

	clm.AssertExpectations(t)
	rm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}
