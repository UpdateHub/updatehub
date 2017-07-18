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
	"path"

	"github.com/OSSystems/pkg/log"
	"github.com/spf13/afero"

	"github.com/UpdateHub/updatehub/copy"
	"github.com/UpdateHub/updatehub/installifdifferent"
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/libarchive"
	"github.com/UpdateHub/updatehub/metadata"
)

func init() {
	installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "raw",
		CheckRequirements: func() error { return nil },
		GetObject: func() interface{} {
			return &RawObject{
				LibArchiveBackend: &libarchive.LibArchive{},
				FileSystemBackend: afero.NewOsFs(),
				CopyBackend:       &copy.ExtendedIO{},
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
	CopyBackend       copy.Interface `json:"-"`
	installifdifferent.TargetGetter

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
	log.Debug("'raw' handler Setup")

	if r.TargetType != "device" {
		finalErr := fmt.Errorf("target-type '%s' is not supported for the 'raw' handler. Its value must be 'device'", r.TargetType)
		log.Error(finalErr)
		return finalErr
	}

	return nil
}

// Install implementation for the "raw" handler
func (r *RawObject) Install(downloadDir string) error {
	log.Debug("'raw' handler Install")

	srcPath := path.Join(downloadDir, r.Sha256sum)
	return r.CopyBackend.CopyFile(r.FileSystemBackend, r.LibArchiveBackend, srcPath, r.Target, r.ChunkSize, r.Skip, r.Seek, r.Count, r.Truncate, r.Compressed)
}

// Cleanup implementation for the "raw" handler
func (r *RawObject) Cleanup() error {
	log.Debug("'raw' handler Cleanup")
	return nil
}

// GetTarget implementation for the "raw" handler
func (r *RawObject) GetTarget() string {
	return r.Target
}
