/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package raw

import (
	"fmt"

	"github.com/spf13/afero"

	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/libarchive"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/utils"
)

func init() {
	installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "raw",
		CheckRequirements: func() error { return nil },
		GetObject: func() interface{} {
			return &RawObject{
				LibArchiveBackend: &libarchive.LibArchive{},
				FileSystemBackend: afero.NewOsFs(),
				Copier:            &utils.ExtendedIO{},
				ChunkSize:         128 * 1024,
				Count:             -1,
				Truncate:          true,
			}
		},
	})
}

// RawObject encapsulates the "raw" handler data and functions
type RawObject struct {
	metadata.ObjectMetadata
	metadata.CompressedObject
	LibArchiveBackend libarchive.API `json:"-"`
	FileSystemBackend afero.Fs
	utils.Copier      `json:"-"`

	Target     string `json:"target"`
	TargetType string `json:"target-type"`
	ChunkSize  int    `json:"chunk-size,omitempty"`
	Skip       int    `json:"skip,omitempty"`
	Seek       int    `json:"seek,omitempty"`
	Count      int    `json:"count,omitempty"`
	Truncate   bool   `json:"truncate,omitempty"`
}

// Setup implementation for the "raw" handler
func (r *RawObject) Setup() error {
	if r.TargetType != "device" {
		return fmt.Errorf("target-type '%s' is not supported for the 'raw' handler. Its value must be 'device'", r.TargetType)
	}

	return nil
}

// Install implementation for the "raw" handler
func (r *RawObject) Install() error {
	// FIXME: on sourcePath we need to: path.Join(r.UpdateDir, r.Sha256sum)
	return r.CopyFile(r.FileSystemBackend, r.LibArchiveBackend, r.Sha256sum, r.Target, r.ChunkSize, r.Skip, r.Seek, r.Count, r.Truncate, r.Compressed)
}

// Cleanup implementation for the "raw" handler
func (r *RawObject) Cleanup() error {
	return nil
}

// FIXME: install-different stuff
