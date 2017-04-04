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

	exists, err := afero.Exists(iid.FileSystemBackend, target)
	if err != nil {
		return false, err
	}

	if !exists {
		return false, fmt.Errorf("install-if-different: target '%s' not found", target)
	}

	sha256sum, ok := o.GetObjectMetadata().InstallIfDifferent.(string)
	if ok {
		// is string, so is a Sha256Sum
		return installIfDifferentSha256Sum(target, sha256sum)
	}

	pattern, ok := o.GetObjectMetadata().InstallIfDifferent.(map[string]interface{})
	if ok {
		// is object, so is a Pattern
		return installIfDifferentPattern(target, pattern)
	}

	return false, fmt.Errorf("unknown install-if-different format")
}

type TargetGetter interface {
	GetTarget() string
}

func installIfDifferentSha256Sum(target string, sha256sum string) (bool, error) {
	// FIXME: implement this
	return false, fmt.Errorf("installIfDifferent: Sha256Sum not yet implemented")
}

func installIfDifferentPattern(target string, pattern map[string]interface{}) (bool, error) {
	// FIXME: implement this
	return false, fmt.Errorf("installIfDifferent: Pattern not yet implemented")
}
