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

func (is *UpdateFetchState) Id() EasyFotaState {
	return is.id
}

func (is *UpdateFetchState) Cancel(ok bool) bool {
	return is.CancellableState.Cancel(ok)
}

func (is *UpdateFetchState) Handle(fota *EasyFota) (State, bool) {
	var nextState State

	nextState = is

	if err := fota.Controller.FetchUpdate(); err == nil {
		return NewInstallUpdateState(), false
	}

	return nextState, false
}
