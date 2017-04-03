/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package handlers

type InstallUpdateHandler interface {
	Setup() error
	Install(downloadDir string) error
	Cleanup() error
}
