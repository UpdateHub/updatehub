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

func (state *InstallUpdateState) Id() EasyFotaState {
	return state.id
}

func (state *InstallUpdateState) Handle(fota *EasyFota) (State, bool) {
	var nextState State

	nextState = state

	return nextState, false
}
