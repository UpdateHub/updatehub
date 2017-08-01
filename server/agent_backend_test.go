/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package server

import (
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/cmdlinemock"
	"github.com/UpdateHub/updatehub/testsmocks/controllermock"
	"github.com/UpdateHub/updatehub/testsmocks/objectmock"
	"github.com/UpdateHub/updatehub/testsmocks/progresstrackermock"
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
ManualMode=true
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

	ab.Settings.ManualMode = false
	routes := ab.Routes()
	assert.Equal(t, 5, len(routes))

	assert.Equal(t, "GET", routes[0].Method)
	assert.Equal(t, "/info", routes[0].Path)
	expectedFunction := reflect.ValueOf(ab.info)
	receivedFunction := reflect.ValueOf(routes[0].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())

	assert.Equal(t, "GET", routes[1].Method)
	assert.Equal(t, "/status", routes[1].Path)
	expectedFunction = reflect.ValueOf(ab.status)
	receivedFunction = reflect.ValueOf(routes[1].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())

	assert.Equal(t, "GET", routes[2].Method)
	assert.Equal(t, "/update/metadata", routes[2].Path)
	expectedFunction = reflect.ValueOf(ab.updateMetadata)
	receivedFunction = reflect.ValueOf(routes[2].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())

	assert.Equal(t, "POST", routes[3].Method)
	assert.Equal(t, "/update/probe", routes[3].Path)
	expectedFunction = reflect.ValueOf(ab.updateProbe)
	receivedFunction = reflect.ValueOf(routes[3].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())

	assert.Equal(t, "GET", routes[4].Method)
	assert.Equal(t, "/log", routes[4].Path)
	expectedFunction = reflect.ValueOf(ab.log)
	receivedFunction = reflect.ValueOf(routes[4].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())

	ab.Settings.ManualMode = true
	routes = ab.Routes()
	assert.Equal(t, 10, len(routes))

	assert.Equal(t, "POST", routes[5].Method)
	assert.Equal(t, "/update", routes[5].Path)
	expectedFunction = reflect.ValueOf(ab.update)
	receivedFunction = reflect.ValueOf(routes[5].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())

	assert.Equal(t, "POST", routes[6].Method)
	assert.Equal(t, "/update/download", routes[6].Path)
	expectedFunction = reflect.ValueOf(ab.updateDownload)
	receivedFunction = reflect.ValueOf(routes[6].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())

	assert.Equal(t, "POST", routes[7].Method)
	assert.Equal(t, "/update/download/abort", routes[7].Path)
	expectedFunction = reflect.ValueOf(ab.updateDownloadAbort)
	receivedFunction = reflect.ValueOf(routes[7].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())

	assert.Equal(t, "POST", routes[8].Method)
	assert.Equal(t, "/update/install", routes[8].Path)
	expectedFunction = reflect.ValueOf(ab.updateInstall)
	receivedFunction = reflect.ValueOf(routes[8].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())

	assert.Equal(t, "POST", routes[9].Method)
	assert.Equal(t, "/reboot", routes[9].Path)
	expectedFunction = reflect.ValueOf(ab.reboot)
	receivedFunction = reflect.ValueOf(routes[9].Handle)
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

	clm.On("Execute", path.Join(metadataPath, "product-uid")).Return([]byte(productUID), nil)
	clm.On("Execute", path.Join(metadataPath, "hardware")).Return([]byte(hardware), nil)
	clm.On("Execute", path.Join(metadataPath, "version")).Return([]byte(firmwareVersion), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-identity.d/key1")).Return([]byte("id1=value1"), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-identity.d/key2")).Return([]byte("id2=value2"), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-attributes.d/attr1")).Return([]byte("attr1=value1"), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-attributes.d/attr2")).Return([]byte("attr2=value2"), nil)

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

func TestStatusRoute(t *testing.T) {
	uh, ab, clm, rm := setup(t)
	defer teardown(t)

	ptm := &progresstrackermock.ProgressTrackerMock{}
	ptm.On("GetProgress").Return(25).Once()

	uh.SetState(updatehub.NewDownloadingState(&metadata.UpdateMetadata{}, ptm))

	jsonMap := map[string]interface{}{}

	jsonMap["status"] = "downloading"
	jsonMap["progress"] = 25

	expectedJSON, err := json.MarshalIndent(jsonMap, "", "    ")
	assert.NoError(t, err)

	rwm := &responsewritermock.ResponseWriterMock{}
	rwm.On("WriteHeader", 200).Return()
	rwm.On("Write", expectedJSON).Return(len(expectedJSON), nil)

	ab.status(rwm, nil, nil)

	clm.AssertExpectations(t)
	ptm.AssertExpectations(t)
	rm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}

func TestUpdateRoute(t *testing.T) {
	uh, ab, clm, rm := setup(t)
	defer teardown(t)

	cm := &controllermock.ControllerMock{}

	uh.Controller = cm

	cm.On("ProbeUpdate", 5).Return((*metadata.UpdateMetadata)(nil), 3600*time.Second).Once()

	uh.TimeStep = time.Hour

	expectedJSON := []byte(`{ "message": "request accepted, update procedure fired" }`)

	rwm := &responsewritermock.ResponseWriterMock{}
	rwm.On("WriteHeader", 202)
	rwm.On("Write", expectedJSON).Return(len(expectedJSON), nil)

	ab.update(rwm, nil, nil)

	<-ab.AllRequestsFinished

	clm.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}

func TestUpdateMetadataRoute(t *testing.T) {
	uh, ab, clm, rm := setup(t)
	defer teardown(t)

	err := afero.WriteFile(uh.Store, path.Join("/tmp/download/", metadata.UpdateMetadataFilename), []byte(validUpdateMetadataWithActiveInactive), 0644)
	assert.NoError(t, err)

	expectedJSON := []byte(validUpdateMetadataWithActiveInactive)

	rwm := &responsewritermock.ResponseWriterMock{}
	rwm.On("WriteHeader", 200)
	rwm.On("Write", expectedJSON).Return(len(expectedJSON), nil)

	ab.updateMetadata(rwm, nil, nil)

	clm.AssertExpectations(t)
	rm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}

func TestUpdateMetadataRouteWithError(t *testing.T) {
	out := map[string]interface{}{}
	out["error"] = fmt.Sprintf("open %s: file does not exist", path.Join("/tmp/download/", metadata.UpdateMetadataFilename))

	expectedResponse, _ := json.MarshalIndent(out, "", "    ")

	_, ab, clm, rm := setup(t)
	defer teardown(t)

	rwm := &responsewritermock.ResponseWriterMock{}
	rwm.On("WriteHeader", 400)
	rwm.On("Write", expectedResponse).Return(len(expectedResponse), nil)

	ab.updateMetadata(rwm, nil, nil)

	clm.AssertExpectations(t)
	rm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}

func TestUpdateProbeRoute(t *testing.T) {
	out := map[string]interface{}{}
	out["update-available"] = false
	out["try-again-in"] = 3600

	expectedResponse, _ := json.MarshalIndent(out, "", "    ")

	uh, ab, clm, rm := setup(t)
	defer teardown(t)

	uh.TimeStep = time.Second
	uh.SetState(updatehub.NewIdleState())

	cm := &controllermock.ControllerMock{}

	uh.Controller = cm
	cm.On("ProbeUpdate", 0).Return((*metadata.UpdateMetadata)(nil), 3600*time.Second)

	rwm := &responsewritermock.ResponseWriterMock{}
	rwm.On("WriteHeader", 200)
	rwm.On("Write", expectedResponse).Return(len(expectedResponse), nil)

	ab.updateProbe(rwm, nil, nil)

	assert.IsType(t, &updatehub.IdleState{}, uh.GetState())

	clm.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}

func TestUpdateDownloadRoute(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	uh, ab, clm, rm := setup(t)
	defer teardown(t)

	cm := &controllermock.ControllerMock{}
	uh.Controller = cm

	repm := &reportermock.ReporterMock{}
	uh.Reporter = repm

	uh.TimeStep = time.Second
	uh.SetState(updatehub.NewIdleState())

	err := afero.WriteFile(uh.Store, path.Join("/tmp/download/", metadata.UpdateMetadataFilename), []byte(validUpdateMetadataWithActiveInactive), 0644)
	assert.NoError(t, err)

	um, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadataWithActiveInactive))
	assert.NoError(t, err)

	repm.On("ReportState", uh.API.Request(), um.PackageUID(), "downloading", "", uh.FirmwareMetadata).Return(nil).Once()
	cm.On("DownloadUpdate", um, mock.Anything, mock.Anything).Return(nil)

	uh.SetState(updatehub.NewIdleState())

	expectedResponse := []byte(`{ "message": "request accepted, downloading update objects" }`)

	rwm := &responsewritermock.ResponseWriterMock{}
	rwm.On("WriteHeader", 202)
	rwm.On("Write", expectedResponse).Return(len(expectedResponse), nil)

	ab.updateDownload(rwm, nil, nil)

	<-ab.AllRequestsFinished

	assert.IsType(t, &updatehub.DownloadedState{}, uh.GetState())

	clm.AssertExpectations(t)
	rm.AssertExpectations(t)
	cm.AssertExpectations(t)
	repm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}

func TestUpdateDownloadRouteWithReadError(t *testing.T) {
	out := map[string]interface{}{}
	out["error"] = fmt.Sprintf("open %s: file does not exist", path.Join("/tmp/download/", metadata.UpdateMetadataFilename))

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

	ab.updateDownload(rwm, nil, nil)

	assert.IsType(t, &updatehub.ErrorState{}, uh.GetState())

	clm.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
	repm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}

func TestUpdateDownloadRouteWithMarshallError(t *testing.T) {
	out := map[string]interface{}{}
	out["error"] = "invalid character 'i' looking for beginning of value"

	expectedResponse, _ := json.MarshalIndent(out, "", "    ")

	uh, ab, clm, rm := setup(t)
	defer teardown(t)

	err := afero.WriteFile(uh.Store, path.Join("/tmp/download/", metadata.UpdateMetadataFilename), []byte("invalid metadata"), 0644)
	assert.NoError(t, err)

	cm := &controllermock.ControllerMock{}
	uh.Controller = cm

	repm := &reportermock.ReporterMock{}
	uh.Reporter = repm

	uh.TimeStep = time.Second
	uh.SetState(updatehub.NewIdleState())

	rwm := &responsewritermock.ResponseWriterMock{}
	rwm.On("WriteHeader", 400)
	rwm.On("Write", expectedResponse).Return(len(expectedResponse), nil)

	ab.updateDownload(rwm, nil, nil)

	assert.IsType(t, &updatehub.ErrorState{}, uh.GetState())

	clm.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
	repm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}

func TestUpdateDownloadAbortRoute(t *testing.T) {
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
	rm.AssertExpectations(t)
	repm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}

func TestUpdateInstallRoute(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	uh, ab, clm, rm := setup(t)
	defer teardown(t)

	cm := &controllermock.ControllerMock{}
	uh.Controller = cm

	repm := &reportermock.ReporterMock{}
	uh.Reporter = repm

	uh.TimeStep = time.Second
	uh.SetState(updatehub.NewIdleState())

	err := afero.WriteFile(uh.Store, path.Join("/tmp/download/", metadata.UpdateMetadataFilename), []byte(validUpdateMetadataWithActiveInactive), 0644)
	assert.NoError(t, err)

	um, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadataWithActiveInactive))
	assert.NoError(t, err)

	repm.On("ReportState", uh.API.Request(), um.PackageUID(), "installing", "", uh.FirmwareMetadata).Return(nil).Once()
	cm.On("InstallUpdate", um, mock.Anything, mock.Anything).Return(nil)

	expectedResponse := []byte(`{ "message": "request accepted, installing update" }`)

	rwm := &responsewritermock.ResponseWriterMock{}
	rwm.On("WriteHeader", 202)
	rwm.On("Write", expectedResponse).Return(len(expectedResponse), nil)

	ab.updateInstall(rwm, nil, nil)

	<-ab.AllRequestsFinished

	assert.IsType(t, &updatehub.InstalledState{}, uh.GetState())

	clm.AssertExpectations(t)
	rm.AssertExpectations(t)
	cm.AssertExpectations(t)
	repm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}

func TestUpdateInstallRouteWithReadError(t *testing.T) {
	out := map[string]interface{}{}
	out["error"] = fmt.Sprintf("open %s: file does not exist", path.Join("/tmp/download/", metadata.UpdateMetadataFilename))

	expectedResponse, _ := json.MarshalIndent(out, "", "    ")

	uh, ab, clm, rm := setup(t)
	defer teardown(t)

	cm := &controllermock.ControllerMock{}

	uh.Controller = cm

	uh.SetState(updatehub.NewIdleState())

	rwm := &responsewritermock.ResponseWriterMock{}
	rwm.On("WriteHeader", 400)
	rwm.On("Write", expectedResponse).Return(len(expectedResponse), nil)

	ab.updateInstall(rwm, nil, nil)

	assert.IsType(t, &updatehub.ErrorState{}, uh.GetState())

	clm.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}

func TestUpdateInstallRouteWithMarshallError(t *testing.T) {
	out := map[string]interface{}{}
	out["error"] = "invalid character 'i' looking for beginning of value"

	expectedResponse, _ := json.MarshalIndent(out, "", "    ")

	uh, ab, clm, rm := setup(t)
	defer teardown(t)

	err := afero.WriteFile(uh.Store, path.Join("/tmp/download/", metadata.UpdateMetadataFilename), []byte("invalid metadata"), 0644)
	assert.NoError(t, err)

	cm := &controllermock.ControllerMock{}

	uh.Controller = cm

	uh.SetState(updatehub.NewIdleState())

	rwm := &responsewritermock.ResponseWriterMock{}
	rwm.On("WriteHeader", 400)
	rwm.On("Write", expectedResponse).Return(len(expectedResponse), nil)

	ab.updateInstall(rwm, nil, nil)

	assert.IsType(t, &updatehub.ErrorState{}, uh.GetState())

	clm.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
	rwm.AssertExpectations(t)
}

func TestRebootRoute(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	uh, ab, clm, rm := setup(t)
	defer teardown(t)

	rm.On("Reboot").Return(nil)

	cm := &controllermock.ControllerMock{}

	uh.Controller = cm

	uh.SetState(updatehub.NewIdleState())

	expectedResponse := []byte(`{ "message": "request accepted, rebooting the device" }`)

	rwm := &responsewritermock.ResponseWriterMock{}
	rwm.On("WriteHeader", 202)
	rwm.On("Write", expectedResponse).Return(len(expectedResponse), nil)

	ab.reboot(rwm, nil, nil)

	<-ab.AllRequestsFinished

	assert.IsType(t, &updatehub.IdleState{}, uh.GetState())

	clm.AssertExpectations(t)
	cm.AssertExpectations(t)
	om.AssertExpectations(t)
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
