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
	"io"
	"os"

	"github.com/OSSystems/pkg/log"
	"github.com/spf13/afero"
	"github.com/updatehub/updatehub/metadata"
	"github.com/updatehub/updatehub/utils"
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

	tg, ok := o.(TargetProvider)
	if !ok {
		// "o" does NOT support install-if-different
		log.Info(fmt.Sprintf("'%s' mode doesn't support install-if-different", mode))
		return true, nil
	}

	// "o" does support install-if-different
	log.Info(fmt.Sprintf("'%s' mode supports install-if-different", mode))

	log.Debug("install-if-different target: ", tg.GetTarget())

	target, err := iid.FileSystemBackend.OpenFile(tg.GetTarget(), os.O_RDONLY, 0)
	if err != nil {
		return false, err
	}
	defer target.Close()

	if _, ok := o.(interface {
		SetupTarget(afero.File)
	}); ok {
		tg.SetupTarget(target)
	}

	switch value := o.GetObjectMetadata().InstallIfDifferent.(type) {
	case string:
		if value == "sha256sum" {
			log.Info("Checking sha256sum")
			// is string, so is a Sha256Sum
			sha256sum := o.GetObjectMetadata().Sha256sum
			return installIfDifferentSha256Sum(iid.FileSystemBackend, target, sha256sum)
		}
		break
	case map[string]interface{}:
		log.Info("checking pattern")
		// is object, so is a Pattern
		var rs io.ReadSeeker

		if o.GetObjectMetadata().Size > 0 {
			rs = utils.LimitReader(target, o.GetObjectMetadata().Size)
		} else {
			rs = target
		}

		return installIfDifferentPattern(iid.FileSystemBackend, rs, value)
	}

	finalErr := fmt.Errorf("unknown install-if-different format")
	log.Error(finalErr)
	return false, finalErr
}

type TargetProvider interface {
	GetTarget() string
	SetupTarget(target afero.File)
}

func installIfDifferentSha256Sum(fsb afero.Fs, target afero.File, sha256sum string) (bool, error) {
	calculatedSha256sum := utils.FsbFileSha256sum(target)

	if calculatedSha256sum == sha256sum {
		log.Info("Sha256sums match. No need to install")
		return false, nil
	}

	log.Info("Sha256sums doesn't match. Installing")
	return true, nil
}

func installIfDifferentPattern(fsb afero.Fs, target io.ReadSeeker, pattern map[string]interface{}) (bool, error) {
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
