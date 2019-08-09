/*
 * UpdateHub
 * Copyright (C) 2019
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package zephyr

import (
	"github.com/spf13/afero"

	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
)

func init() {
	installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "zephyr",
		CheckRequirements: func() error { return nil },
		GetObject: func() interface{} {
			return &ZephyrObject{}
		},
	})
}

// ZephyrObject encapsulates the "zephyr" handler data and functions
type ZephyrObject struct {
	metadata.ObjectMetadata
}

// Setup implementation for the "zephyr" handler
func (m *ZephyrObject) Setup() error {
	panic("Not supported")
}

// Install implementation for the "zephyr" handler
func (m *ZephyrObject) Install(downloadDir string) error {
	panic("Not supported")
}

// Cleanup implementation for the "zephyr" handler
func (m *ZephyrObject) Cleanup() error {
	panic("Not supported")
}

// GetTarget implementation for the "zephyr" handler
func (m *ZephyrObject) GetTarget() string {
	panic("Not supported")
}

// SetupTarget implementation for the "zephyr" handler
func (m *ZephyrObject) SetupTarget(target afero.File) {
	panic("Not supported")
}
