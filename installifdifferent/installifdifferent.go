/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package installifdifferent

import (
	"fmt"

	"github.com/OSSystems/pkg/log"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/utils"
	"github.com/spf13/afero"
)

type Interface interface {
	Proceed(o metadata.Object) (bool, error)
}

type DefaultImpl struct {
	FileSystemBackend afero.Fs
}

func (iid *DefaultImpl) Proceed(o metadata.Object) (bool, error) {
	if o.GetObjectMetadata().InstallIfDifferent == nil {
		return true, nil
	}

	mode := o.GetObjectMetadata().Mode
	log.Info(fmt.Sprintf("checking install-if-different support for '%s'", mode))

	tg, ok := o.(TargetGetter)
	if !ok {
		// "o" does NOT support install-if-different
		log.Info(fmt.Sprintf("'%s' mode doesn't support install-if-different", mode))
		return true, nil
	}

	// "o" does support install-if-different
	log.Info(fmt.Sprintf("'%s' mode supports install-if-different", mode))

	target := tg.GetTarget()
	log.Debug("install-if-different target: ", target)

	sha256sum, ok := o.GetObjectMetadata().InstallIfDifferent.(string)
	if ok {
		log.Info("Checking sha256sum")
		// is string, so is a Sha256Sum
		return installIfDifferentSha256Sum(iid.FileSystemBackend, target, sha256sum)
	}

	pattern, ok := o.GetObjectMetadata().InstallIfDifferent.(map[string]interface{})
	if ok {
		log.Info("checking pattern")
		// is object, so is a Pattern
		return installIfDifferentPattern(iid.FileSystemBackend, target, pattern)
	}

	finalErr := fmt.Errorf("unknown install-if-different format")
	log.Error(finalErr)
	return false, finalErr
}

type TargetGetter interface {
	GetTarget() string
}

func installIfDifferentSha256Sum(fsb afero.Fs, target string, sha256sum string) (bool, error) {
	calculatedSha256sum, err := utils.FileSha256sum(fsb, target)
	if err != nil {
		finalErr := fmt.Errorf("failed to check sha256sums: %s", err)
		log.Error(finalErr)
		return false, finalErr
	}

	if calculatedSha256sum == sha256sum {
		log.Info("Sha256sums match. No need to install")
		return false, nil
	}

	log.Info("Sha256sums doesn't match. Installing")
	return true, nil
}

func installIfDifferentPattern(fsb afero.Fs, target string, pattern map[string]interface{}) (bool, error) {
	p, err := NewPatternFromInstallIfDifferentObject(fsb, pattern)
	if err != nil {
		finalErr := fmt.Errorf("failed to parse install-if-different object: %s", err)
		log.Error(finalErr)
		return false, finalErr
	}

	if p.IsValid() {
		capturedVersion, err := p.Capture(target)

		if err != nil {
			return false, err
		}

		if capturedVersion != "" {
			install := pattern["version"].(string) != capturedVersion

			if install {
				log.Info("Version mismatch. Installing")
				return true, nil
			} else {
				log.Info("Version match. No need to install")
				return false, nil
			}
		}
	}

	return false, nil
}
