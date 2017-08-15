/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package utils

import (
	"net/url"
	"strings"
)

type Rebooter interface {
	Reboot() error
}

type RebooterImpl struct {
}

func (r *RebooterImpl) Reboot() error {
	c := &CmdLine{}

	_, err := c.Execute("/sbin/reboot")

	return err
}

func SanitizeServerAddress(address string) (string, error) {
	a := address
	if !strings.HasPrefix(a, "http://") && !strings.HasPrefix(a, "https://") {
		a = "https://" + a
	}

	serverURL, err := url.Parse(a)
	if err != nil {
		return "", err
	}

	return serverURL.String(), nil
}
