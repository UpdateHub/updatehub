package main

import (
	"time"

	"bitbucket.org/ossystems/agent/client"
	_ "bitbucket.org/ossystems/agent/installmodes/copy"
)

func main() {
	fota := &EasyFota{
		state:        NewPollState(),
		pollInterval: 5,
		api:          client.NewApiClient("localhost:8080"),
		updater:      client.NewUpdateClient(),
		timeStep:     time.Minute,
	}

	fota.Controller = fota

	fota.MainLoop()
}
