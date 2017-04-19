/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package installifdifferent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/spf13/afero"
)

type Interface interface {
	Proceed(o metadata.Object) (bool, error)
}

type DefaultImpl struct {
	FileSystemBackend afero.Fs
}

func (iid *DefaultImpl) Proceed(o metadata.Object) (bool, error) {
	om, err := installmodes.GetObject(o.GetObjectMetadata().Mode)
	if err != nil {
		return false, err
	}

	tg, ok := om.(TargetGetter)

	if !ok {
		// "o" does NOT support install-if-different
		return true, nil
	}

	// "o" does support install-if-different

	target := tg.GetTarget()

	sha256sum, ok := o.GetObjectMetadata().InstallIfDifferent.(string)
	if ok {
		// is string, so is a Sha256Sum
		return installIfDifferentSha256Sum(iid.FileSystemBackend, target, sha256sum)
	}

	pattern, ok := o.GetObjectMetadata().InstallIfDifferent.(map[string]interface{})
	if ok {
		// is object, so is a Pattern
		return installIfDifferentPattern(iid.FileSystemBackend, target, pattern)
	}

	return false, fmt.Errorf("unknown install-if-different format")
}

type TargetGetter interface {
	GetTarget() string
}

func installIfDifferentSha256Sum(fsb afero.Fs, target string, sha256sum string) (bool, error) {
	file, err := fsb.Open(target)
	if err != nil {
		return false, err
	}

	hash := sha256.New()
	_, err = io.Copy(hash, file)
	calculatedSha256sum := hex.EncodeToString(hash.Sum(nil))

	file.Close()

	if calculatedSha256sum == sha256sum {
		return false, nil
	}

	return true, nil
}

func installIfDifferentPattern(fsb afero.Fs, target string, pattern map[string]interface{}) (bool, error) {
	p, err := NewPatternFromInstallIfDifferentObject(fsb, pattern)
	if err != nil {
		return false, err
	}

	if p.IsValid() {
		capturedVersion, err := p.Capture(target)

		if err != nil {
			return false, err
		}

		if capturedVersion != "" {
			install := pattern["version"].(string) != capturedVersion
			return install, nil
		}
	}

	return false, nil
}
