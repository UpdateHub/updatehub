package main

import "time"

type IdleState struct {
	BaseState
	CancellableState

	elapsedTime int
}

func NewIdleState() *IdleState {
	state := &IdleState{
		BaseState:        BaseState{id: EasyFotaStateIdle},
		CancellableState: CancellableState{cancel: make(chan bool)},
	}
	return state
}

func (state *IdleState) Id() EasyFotaState {
	return state.id
}

func (state *IdleState) Cancel(ok bool) bool {
	return state.CancellableState.Cancel(ok)
}

func (state *IdleState) Handle(fota *EasyFota) (State, bool) {
	var nextState State

	nextState = state

	go func() {
		for {
			if state.elapsedTime == fota.pollInterval {
				state.elapsedTime = 0
				nextState = NewUpdateCheckState()
				break
			}

			time.Sleep(time.Second)

			state.elapsedTime++
		}

		state.Cancel(true)
	}()

	state.Wait()

	return nextState, false
}
