/*
 * UpdateHub
 * Copyright (C) 2018
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package mender

import (
	"github.com/spf13/afero"

	"github.com/updatehub/updatehub/installmodes"
	"github.com/updatehub/updatehub/metadata"
)

func init() {
	installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "mender",
		CheckRequirements: func() error { return nil },
		GetObject: func() interface{} {
			return &MenderObject{}
		},
	})
}

// MenderObject encapsulates the "mender" handler data and functions
type MenderObject struct {
	metadata.ObjectMetadata
}

// Setup implementation for the "mender" handler
func (m *MenderObject) Setup() error {
	return nil
}

// Install implementation for the "mender" handler
func (m *MenderObject) Install(downloadDir string) error {
	return nil
}

// Cleanup implementation for the "mender" handler
func (m *MenderObject) Cleanup() error {
	return nil
}

// GetTarget implementation for the "mender" handler
func (m *MenderObject) GetTarget() string {
	return ""
}

// SetupTarget implementation for the "mender" handler
func (m *MenderObject) SetupTarget(target afero.File) {
}
