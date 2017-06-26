/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package utils

import "syscall"

type Rebooter interface {
	Reboot() error
}

type RebooterImpl struct {
}

func (r *RebooterImpl) Reboot() error {
	return syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
}
