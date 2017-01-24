package main

import (
	"bitbucket.org/ossystems/agent/client"
	_ "bitbucket.org/ossystems/agent/installmodes/copy"
)

func main() {
	fota := &EasyFota{
		state:        NewIdleState(),
		pollInterval: 5,
		api:          client.NewApiClient("localhost:8080"),
		updater:      client.NewUpdateClient(),
	}

	fota.Controller = fota

	fota.MainLoop()
}
