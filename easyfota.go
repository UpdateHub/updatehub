package main

import "fmt"

type EasyFota struct {
	state        State
	pollInterval int
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
