package main

import "fmt"

type EasyFota struct {
	state        State
	pollInterval int
}

func (ef *EasyFota) MainLoop() {
	for {
		fmt.Println("Handling state:", StateToString(ef.state.Id()))

		state := ef.state.Handle(ef)

		ef.state = state
	}
}
