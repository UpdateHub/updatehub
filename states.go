package main

import (
	"errors"
	"time"

	"bitbucket.org/ossystems/agent/metadata"
)

type EasyFotaState int

const (
	EasyFotaStatePoll = iota
	EasyFotaStateUpdateCheck
	EasyFotaStateUpdateFetch
	EasyFotaStateUpdateInstall
	EasyFotaStateInstalling
	EasyFotaStateInstalled
	EasyFotaStateWaitingForReboot
	EasyFotaStateError
)

var statusNames = map[EasyFotaState]string{
	EasyFotaStatePoll:             "poll",
	EasyFotaStateUpdateCheck:      "update-check",
	EasyFotaStateUpdateFetch:      "update-fetch",
	EasyFotaStateUpdateInstall:    "update-install",
	EasyFotaStateInstalling:       "installing",
	EasyFotaStateInstalled:        "installed",
	EasyFotaStateWaitingForReboot: "waiting-for-reboot",
	EasyFotaStateError:            "error",
}

type BaseState struct {
	id EasyFotaState
}

func (b *BaseState) Id() EasyFotaState {
	return b.id
}

func (b *BaseState) Cancel(ok bool) bool {
	return ok
}

type State interface {
	Id() EasyFotaState
	Handle(*EasyFota) (State, bool)
	Cancel(bool) bool
}

func StateToString(status EasyFotaState) string {
	return statusNames[status]
}

type ErrorState struct {
	BaseState
	cause EasyFotaErrorReporter
}

func (state *ErrorState) Handle(fota *EasyFota) (State, bool) {
	if state.cause.IsFatal() {
		panic(state.cause)
	}

	return NewPollState(), false
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

type PollState struct {
	BaseState
	CancellableState

	elapsedTime int
	extraPoll   int
	ticksCount  int
}

func (state *PollState) Id() EasyFotaState {
	return state.id
}

func (state *PollState) Cancel(ok bool) bool {
	return state.CancellableState.Cancel(ok)
}

func (state *PollState) Handle(fota *EasyFota) (State, bool) {
	var nextState State

	nextState = state

	go func() {
		for {
			if state.ticksCount > 0 && state.ticksCount%(fota.pollInterval+state.extraPoll) == 0 {
				state.extraPoll = 0
				nextState = NewUpdateCheckState()
				break
			}

			time.Sleep(fota.timeStep)

			state.ticksCount++
		}

		state.Cancel(true)
	}()

	state.Wait()

	return nextState, false
}

func NewPollState() *PollState {
	state := &PollState{
		BaseState:        BaseState{id: EasyFotaStatePoll},
		CancellableState: CancellableState{cancel: make(chan bool)},
	}

	return state
}

type UpdateCheckState struct {
	BaseState
}

func (state *UpdateCheckState) Id() EasyFotaState {
	return state.id
}

func (state *UpdateCheckState) Handle(fota *EasyFota) (State, bool) {
	updateMetadata, extraPoll := fota.Controller.CheckUpdate()

	if updateMetadata != nil {
		return NewUpdateFetchState(updateMetadata), false
	}

	poll := NewPollState()

	if extraPoll > 0 {
		poll.extraPoll = extraPoll

		return poll, false
	}

	return poll, false
}

func NewUpdateCheckState() *UpdateCheckState {
	state := &UpdateCheckState{
		BaseState: BaseState{id: EasyFotaStateUpdateCheck},
	}

	return state
}

type UpdateFetchState struct {
	BaseState
	CancellableState

	updateMetadata *metadata.Metadata
}

func (state *UpdateFetchState) Id() EasyFotaState {
	return state.id
}

func (state *UpdateFetchState) Cancel(ok bool) bool {
	state.CancellableState.Cancel(ok)
	return ok
}

func (state *UpdateFetchState) Handle(fota *EasyFota) (State, bool) {
	var nextState State

	nextState = state

	if err := fota.Controller.FetchUpdate(state.updateMetadata, state.cancel); err == nil {
		return NewInstallUpdateState(), false
	}

	return nextState, false
}

func NewUpdateFetchState(updateMetadata *metadata.Metadata) *UpdateFetchState {
	state := &UpdateFetchState{
		BaseState:      BaseState{id: EasyFotaStateUpdateFetch},
		updateMetadata: updateMetadata,
	}

	return state
}

type InstallUpdateState struct {
	BaseState
}

func (state *InstallUpdateState) Id() EasyFotaState {
	return state.id
}

func (state *InstallUpdateState) Handle(fota *EasyFota) (State, bool) {
	var nextState State

	nextState = state

	return nextState, false
}

func NewInstallUpdateState() *InstallUpdateState {
	state := &InstallUpdateState{
		BaseState: BaseState{id: EasyFotaStateUpdateInstall},
	}

	return state
}
