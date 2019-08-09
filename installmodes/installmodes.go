/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package installmodes

import "errors"

var (
	installModes = make(map[string]InstallMode)
)

// InstallMode represents a install mode
type InstallMode struct {
	Name              string
	CheckRequirements func() error
	GetObject         func() interface{}
}

func (mode InstallMode) Unregister() {
	delete(installModes, mode.Name)
}

// RegisterInstallMode registers a new install mode
func RegisterInstallMode(mode InstallMode) InstallMode {
	installModes[mode.Name] = mode
	return mode
}

// GetObject gets the object that represents a install mode
func GetObject(name string) (interface{}, error) {
	if m, ok := installModes[name]; ok {
		return m.GetObject(), nil
	} else {
		return nil, errors.New("Object not found")
	}
}

// CheckRequirements iterates over all registered install modes and check for their requirements
func CheckRequirements() error {
	for _, m := range installModes {
		if err := m.CheckRequirements(); err != nil {
			return err
		}
	}

	return nil
}
