package main

import (
	_ "bitbucket.org/ossystems/agent/installmodes/copy"
)

func main() {
	fota := EasyFota{
		state:        NewIdleState(),
		pollInterval: 5,
	}

	fota.MainLoop()
}
