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
	"strings"

	"github.com/OSSystems/pkg/log"
	"github.com/updatehub/updatehub/utils"
)

// Interface describes the operations related to the Active-Inactive feature
type Interface interface {
	Active() (int, error)
	SetActive(active int) error
	SetValidate() error
}

// DefaultImpl is the default implementation for Interface
type DefaultImpl struct {
	utils.CmdLineExecuter
}

// Active returns the current active object number
func (i *DefaultImpl) Active() (int, error) {
	log.Debug("Running 'updatehub-active-get'")

	output, err := i.Execute("updatehub-active-get")
	if err != nil {
		finalErr := fmt.Errorf("failed to execute 'updatehub-active-get': %s", err)
		log.Error(finalErr)
		return 0, finalErr
	}

	activeIndex, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 0)
	if err != nil {
		finalErr := fmt.Errorf("failed to parse response from 'updatehub-active-get': %s", err)
		log.Error(finalErr)
		return 0, finalErr
	}

	log.Debug("Active partition: ", int(activeIndex))

	return int(activeIndex), nil
}

// SetActive sets the current active object number to "active"
func (i *DefaultImpl) SetActive(active int) error {
	log.Debug("Running 'updatehub-active-set' for partition: ", active)

	_, err := i.Execute(fmt.Sprintf("updatehub-active-set %d", active))
	if err != nil {
		finalErr := fmt.Errorf("failed to execute 'updatehub-active-set': %s", err)
		log.Error(finalErr)
		return finalErr
	}

	return nil
}

// SetValidate validate the current update
// by calling 'updatehub-active-validated'
func (i *DefaultImpl) SetValidate() error {
	log.Debug("Running 'updatehub-active-validated'")

	_, err := i.Execute(fmt.Sprintf("updatehub-active-validated"))
	if err != nil {
		finalErr := fmt.Errorf("failed to execute 'updatehub-active-validated': %s", err)
		log.Error(finalErr)
		return finalErr
	}

	return nil
}
