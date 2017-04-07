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
	systemSettingsPath = "/etc/updatehub-agent.conf"
	// The runtime settings are the settings that may can change during the execution of UpdateHub
	// These settings are persisted to keep the behaviour across of device's reboot
	runtimeSettingsPath = "/var/lib/updatehub-agent.conf"
	// The path on which will be located the scripts that provide the firmware metadata
	firmwareMetadataDirPath = "/usr/share/updatehub-agent"
)
