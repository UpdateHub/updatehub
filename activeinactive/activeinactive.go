/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package activeinactive

import (
	"fmt"
	"strconv"

	"github.com/UpdateHub/updatehub/utils"
)

// Interface describes the operations related to the Active-Inactive feature
type Interface interface {
	Active() (int, error)
	SetActive(active int) error
}

// DefaultImpl is the default implementation for Interface
type DefaultImpl struct {
	utils.CmdLineExecuter
}

// Active returns the current active object number
func (i *DefaultImpl) Active() (int, error) {
	output, err := i.Execute("updatehub-active-get")
	if err != nil {
		return 0, err
	}

	activeIndex, err := strconv.ParseInt(string(output), 10, 0)
	if err != nil {
		return 0, err
	}

	return int(activeIndex), nil
}

// SetActive sets the current active object number to "active"
func (i *DefaultImpl) SetActive(active int) error {
	_, err := i.Execute(fmt.Sprintf("updatehub-active-set %d", active))
	if err != nil {
		return err
	}

	return nil
}
