package main

import "fmt"

type EasyFota struct {
	Controller

	state        State
	pollInterval int
}

type Controller interface {
	CheckUpdate() bool
	FetchUpdate() error
}

func (fota *EasyFota) CheckUpdate() bool {
	return false
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
