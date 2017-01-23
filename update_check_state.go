package main

import "fmt"

type UpdateCheckState struct {
	BaseState
}

func NewUpdateCheckState() *UpdateCheckState {
	state := &UpdateCheckState{
		BaseState: BaseState{id: EasyFotaStateUpdateCheck},
	}
	return state
}

func (is *UpdateCheckState) Id() EasyFotaState {
	return is.id
}

func (is *UpdateCheckState) Handle(fota *EasyFota) (State, bool) {
	if fota.Controller.CheckUpdate() {
		return NewUpdateFetchState(), false
	}

	fmt.Println("No update available")

	return NewIdleState(), false
}
