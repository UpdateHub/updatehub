package main

type UpdateFetchState struct {
	BaseState
	CancellableState
}

func NewUpdateFetchState() *UpdateFetchState {
	state := &UpdateFetchState{
		BaseState:        BaseState{id: EasyFotaStateIdle},
		CancellableState: CancellableState{cancel: make(chan bool)},
	}
	return state
}

func (state *UpdateFetchState) Id() EasyFotaState {
	return state.id
}

func (state *UpdateFetchState) Cancel(ok bool) bool {
	return state.CancellableState.Cancel(ok)
}

func (state *UpdateFetchState) Handle(fota *EasyFota) (State, bool) {
	var nextState State

	nextState = state

	if err := fota.Controller.FetchUpdate(); err == nil {
		return NewInstallUpdateState(), false
	}

	return nextState, false
}
