package main

import "fmt"

type EasyFota struct {
	Controller

	state        State
	pollInterval int
}

type Controller interface {
	CheckUpdate() bool
}

func (ef *EasyFota) CheckUpdate() bool {
	return false
}

func (ef *EasyFota) MainLoop() {
	for {
		fmt.Println("Handling state:", StateToString(ef.state.Id()))

		state, cancelled := ef.state.Handle(ef)

		if cancelled {
			fmt.Println("State cancelled")
		}

		ef.state = state
	}
}
