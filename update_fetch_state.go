package main

import "time"

type UpdateFetchState struct {
	BaseState
	CancellableState

	elapsedTime int
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

	go func() {
		for {
			if is.elapsedTime == fota.pollInterval {
				is.elapsedTime = 0
				nextState = NewUpdateCheckState()
				break
			}

			time.Sleep(time.Second)

			is.elapsedTime++
		}

		is.Cancel(true)
	}()

	is.Wait()

	return nextState, false
}
