/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package utils

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
