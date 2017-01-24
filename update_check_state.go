package main

type UpdateCheckState struct {
	BaseState
}

func NewUpdateCheckState() *UpdateCheckState {
	state := &UpdateCheckState{
		BaseState: BaseState{id: EasyFotaStateUpdateCheck},
	}
	return state
}

func (state *UpdateCheckState) Id() EasyFotaState {
	return state.id
}

func (state *UpdateCheckState) Handle(fota *EasyFota) (State, bool) {
	if fota.Controller.CheckUpdate() {
		return NewUpdateFetchState(), false
	}

	return NewIdleState(), false
}
