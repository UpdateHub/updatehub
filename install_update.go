/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import "github.com/UpdateHub/updatehub/handlers"

func InstallUpdate(h handlers.InstallUpdateHandler) error {
	if err := h.Setup(); err != nil {
		return err
	}

	if err := h.Install(); err != nil {
		return err
	}

	if err := h.Cleanup(); err != nil {
		return err
	}

	return nil
}
