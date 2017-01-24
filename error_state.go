package main

import "errors"

type ErrorState struct {
	BaseState
	cause EasyFotaErrorReporter
}

func (state *ErrorState) Handle(fota *EasyFota) (State, bool) {
	if state.cause.IsFatal() {
		panic(state.cause)
	}

	return NewIdleState(), false
}

func NewErrorState(err EasyFotaErrorReporter) State {
	if err == nil {
		err = NewFatalError(errors.New("generic error"))
	}

	return &ErrorState{
		BaseState: BaseState{id: EasyFotaStateError},
		cause:     err,
	}
}
