package main

import (
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/updatehub"
)

type AgentInfo struct {
	Version   string                    `json:"version"`
	Config    updatehub.Settings        `json:"config"`
	Firmware  metadata.FirmwareMetadata `json:"firmware"`
}

type LogEntry struct {
	Data    interface{} `json:"data"`
	Level   string      `json:"level"`
	Message string      `json:"message"`
	Time    string      `json:"time"`
}

type ProbeResponse struct {
	UpdateAvailable bool `json:"update-available"`
	TryAgainIn      int  `json:"try-again-in"`
}
