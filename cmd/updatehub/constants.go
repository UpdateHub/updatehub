/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

const (
	// The system settings are the settings configured in the client-side and will be read-only
	systemSettingsPath = "/etc/updatehub.conf"

	// The state change callback is executed twice each state
	// change. Once before the state handling and once after. Ex.:
	// <stateChangeCallbackPath> enter downloading
	// <stateChangeCallbackPath> leave downloading
	stateChangeCallbackPath = "/usr/share/updatehub/state-change-callback"

	// The error callback is executed whenever a error state is
	// handled. Ex.:
	// <errorCallbackPath> 'error_message'
	errorCallbackPath = "/usr/share/updatehub/error-callback"
)
