/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
	"github.com/UpdateHub/updatehub/testsmocks/filemock"
	"github.com/UpdateHub/updatehub/testsmocks/filesystemmock"
	"github.com/UpdateHub/updatehub/updatehub"
	"github.com/go-ini/ini"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestLoadSettings(t *testing.T) {
	fs := afero.NewOsFs()

	testPath, err := afero.TempDir(fs, "", "updatehub-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	runtimeSettingsTestPath := path.Join(testPath, "runtime.conf")
	systemSettingsTestPath := path.Join(testPath, "system.conf")

	testCases := []struct {
		name             string
		systemSettings   string
		runtimeSettings  string
		expectedError    interface{}
		expectedSettings updatehub.Settings
	}{

		{
			"InvalidSettingsFile",
			"test",
			"test",
			ini.ErrDelimiterNotFound{Line: "test"},
			updatehub.Settings{},
		},

		{
			"ValidSettingsFile",
			"[Storage]\nReadOnly=true",
			"[Polling]\nExtraInterval=3",
			nil,
			func() updatehub.Settings {
				s := updatehub.DefaultSettings

				s.ReadOnly = false
				s.ExtraPollingInterval = 3
				s.LastPoll = (time.Time{}).UTC()
				s.FirstPoll = (time.Time{}).UTC()

				return s
			}(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			aim := &activeinactivemock.ActiveInactiveMock{}

			if tc.systemSettings != "" {
				err := fs.MkdirAll(filepath.Dir(systemSettingsTestPath), 0755)
				assert.NoError(t, err)
				err = afero.WriteFile(fs, systemSettingsTestPath, []byte(tc.systemSettings), 0644)
				assert.NoError(t, err)
			}

			if tc.runtimeSettings != "" {
				err := fs.MkdirAll(filepath.Dir(runtimeSettingsTestPath), 0755)
				assert.NoError(t, err)
				err = afero.WriteFile(fs, runtimeSettingsTestPath, []byte(tc.runtimeSettings), 0644)
				assert.NoError(t, err)
			}

			settings := &updatehub.Settings{}

			err := loadSettings(fs, settings, systemSettingsTestPath)
			assert.Equal(t, tc.expectedError, err)

			err = loadSettings(fs, settings, runtimeSettingsTestPath)
			assert.Equal(t, tc.expectedError, err)

			assert.Equal(t, tc.expectedSettings, *settings)

			dirExists, err := afero.Exists(fs, filepath.Dir(systemSettingsTestPath))
			assert.True(t, dirExists)
			assert.NoError(t, err)

			dirExists, err = afero.Exists(fs, filepath.Dir(runtimeSettingsTestPath))
			assert.True(t, dirExists)
			assert.NoError(t, err)

			aim.AssertExpectations(t)
		})
	}
}

func TestLoadSettingsWithOpenError(t *testing.T) {
	fsbm := &filesystemmock.FileSystemBackendMock{}

	settings := &updatehub.Settings{}

	settingsPath := "/path/subdir"

	fsbm.On("MkdirAll", "/path", os.FileMode(0755)).Return(nil)
	fsbm.On("Open", settingsPath).Return((*filemock.FileMock)(nil), fmt.Errorf("open error"))

	err := loadSettings(fsbm, settings, settingsPath)
	assert.EqualError(t, err, "open error")

	fsbm.AssertExpectations(t)
}
