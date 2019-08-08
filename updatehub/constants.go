/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

const (
	// The system settings are the settings configured in the client-side and will be read-only
	SystemSettingsPath = "/etc/updatehub.conf"

	// The state change callback is executed twice each state
	// change. Once before the state handling and once after. Ex.:
	// <stateChangeCallbackPath> enter downloading
	// <stateChangeCallbackPath> leave downloading
	StateChangeCallbackPath = "/usr/share/updatehub/state-change-callback"

	// The error callback is executed whenever a error state is
	// handled. Ex.:
	// <errorCallbackPath> 'error_message'
	ErrorCallbackPath = "/usr/share/updatehub/error-callback"

	// The validate callback is executed whenever a successful
	// installation is booted.
	ValidateCallbackPath = "/usr/share/updatehub/validate-callback"

	// The rollback callback is executed whenever the agent boots
	// after an errored installation boot
	RollbackCallbackPath = "/usr/share/updatehub/rollback-callback"
)
