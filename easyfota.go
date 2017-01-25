package main

import (
	"fmt"
	"time"

	"bitbucket.org/ossystems/agent/client"
)

type EasyFota struct {
	Controller

	state        State
	pollInterval int
	timeStep     time.Duration
	api          *client.ApiClient
	updater      client.Updater
}

type Controller interface {
	CheckUpdate() (bool, int)
	FetchUpdate() error
}

func (fota *EasyFota) CheckUpdate() (bool, int) {
	_, extraPoll, err := fota.updater.CheckUpdate(fota.api.Request())
	if err != nil {
		return false, 0
	}

	return true, extraPoll
}

func (fota *EasyFota) FetchUpdate() error {
	return nil
}

func (fota *EasyFota) MainLoop() {
	for {
		fmt.Println("Handling state:", StateToString(fota.state.Id()))

		state, cancelled := fota.state.Handle(fota)

		if state.Id() == EasyFotaStateError {
			if es, ok := state.(*ErrorState); ok {
				// FIXME: log error
				fmt.Println(es.cause)
			}
		}

		if cancelled {
			fmt.Println("State cancelled")
		}

		fota.state = state
	}
}
