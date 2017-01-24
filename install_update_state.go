package main

type InstallUpdateState struct {
	BaseState
}

func NewInstallUpdateState() *InstallUpdateState {
	state := &InstallUpdateState{
		BaseState: BaseState{id: EasyFotaStateIdle},
	}
	return state
}

func (is *InstallUpdateState) Id() EasyFotaState {
	return is.id
}

func (is *InstallUpdateState) Handle(fota *EasyFota) (State, bool) {
	var nextState State

	nextState = is

	return nextState, false
}
