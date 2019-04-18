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
	"net/http/httptest"
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

	_, err := NewAgentBackend(uh)
	assert.NoError(t, err)
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
	jsonMap["config"] = uh.Settings
	jsonMap["firmware"] = uh.FirmwareMetadata

	expectedJSON, err := json.MarshalIndent(jsonMap, "", "    ")
	assert.NoError(t, err)

	router := NewRouter(ab)
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/info", nil)

	router.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code)
	assert.Equal(t, bytes.NewBuffer(expectedJSON), rr.Body)

	clm.AssertExpectations(t)
	rm.AssertExpectations(t)
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
	cm.On("ProbeUpdate", uh.DefaultApiClient, 5).Return((*metadata.UpdateMetadata)(nil), []byte{}, 3600*time.Second, nil)

	done := make(chan bool, 1)
	go func() {
		ok := false
		for ok == false {
			_, ok = s.NextState().(*updatehub.ProbeState)
			time.Sleep(100 * time.Millisecond)
		}

		s.NextState().Handle(uh)

		done <- true
	}()

	router := NewRouter(ab)
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/probe", bytes.NewBuffer([]byte(`{}`)))
	router.ServeHTTP(rr, req)

	<-done

	assert.Equal(t, 200, rr.Code)
	assert.JSONEq(t, string(expectedResponse), rr.Body.String())

	assert.IsType(t, &updatehub.ProbeState{}, s.NextState())

	clm.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
}

func TestProbeRouteIsBusy(t *testing.T) {
	out := map[string]interface{}{}
	out["busy"] = true
	out["current-state"] = "rebooting"

	expectedResponse, _ := json.MarshalIndent(out, "", "    ")

	uh, ab, _, _ := setup(t)
	defer teardown(t)

	s := updatehub.NewRebootingState(uh.DefaultApiClient, nil)
	uh.SetState(s)

	router := NewRouter(ab)
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/probe", bytes.NewBuffer([]byte(`{}`)))
	router.ServeHTTP(rr, req)

	assert.Equal(t, 202, rr.Code)
	assert.JSONEq(t, string(expectedResponse), rr.Body.String())
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
			// Not needed for this test case
			apiClient.CheckRedirect = nil

			uh.TimeStep = time.Second
			s := updatehub.NewIdleState()
			uh.SetState(s)

			cm := &controllermock.ControllerMock{}

			uh.Controller = cm
			cm.On("ProbeUpdate", mock.MatchedBy(func(actual *client.ApiClient) bool {
				// Not needed for this test case
				actual.CheckRedirect = nil
				return reflect.DeepEqual(actual, apiClient)
			}), 5).Return((*metadata.UpdateMetadata)(nil), []byte{}, 3600*time.Second, nil)

			done := make(chan bool, 1)
			go func() {
				ok := false
				for ok == false {
					_, ok = s.NextState().(*updatehub.ProbeState)
					time.Sleep(100 * time.Millisecond)
				}

				s.NextState().Handle(uh)

				done <- true
			}()

			body := bytes.NewBufferString(fmt.Sprintf(`{ "server-address": "%s" }`, tc.ExpectedURL))

			ab.DefaultApiClient = apiClient

			router := NewRouter(ab)
			rr := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/probe", body)
			router.ServeHTTP(rr, req)

			<-done

			assert.IsType(t, &updatehub.ProbeState{}, s.NextState())

			assert.Equal(t, 200, rr.Code)
			assert.JSONEq(t, string(expectedResponse), rr.Body.String())

			clm.AssertExpectations(t)
			cm.AssertExpectations(t)
			rm.AssertExpectations(t)
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

	router := NewRouter(ab)
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/update/download/abort", nil)
	router.ServeHTTP(rr, req)

	assert.IsType(t, &updatehub.IdleState{}, ds.NextState())

	assert.Equal(t, 200, rr.Code)
	assert.JSONEq(t, string(expectedJSON), rr.Body.String())

	clm.AssertExpectations(t)
	cm.AssertExpectations(t)
	repm.AssertExpectations(t)
	rm.AssertExpectations(t)
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

	router := NewRouter(ab)
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/update/download/abort", nil)
	router.ServeHTTP(rr, req)

	assert.IsType(t, &updatehub.ErrorState{}, uh.GetState())

	assert.Equal(t, 400, rr.Code)
	assert.JSONEq(t, string(expectedResponse), rr.Body.String())

	clm.AssertExpectations(t)
	cm.AssertExpectations(t)
	repm.AssertExpectations(t)
	rm.AssertExpectations(t)
}

func TestLogRoute(t *testing.T) {
	_, ab, clm, rm := setup(t)
	defer teardown(t)

	router := NewRouter(ab)
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/log", nil)
	router.ServeHTTP(rr, req)

	jsonArray := []map[string]interface{}{}
	err := json.NewDecoder(rr.Body).Decode(&jsonArray)
	assert.NoError(t, err)

	assert.Equal(t, 200, rr.Code)
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
}
