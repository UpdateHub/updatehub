/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

// RebootState is the State interface implementation for the UpdateHubStateReboot
type RebootState struct {
	BaseState
}

// ID returns the state id
func (state *RebootState) ID() UpdateHubState {
	return state.id
}

// Handle for RebootState implements the installation process itself
func (state *RebootState) Handle(uh *UpdateHub) (State, bool) {
	err := uh.Reboot()
	if err != nil {
		return NewErrorState(nil, NewTransientError(err)), false
	}

	return NewIdleState(), false
}

// NewRebootState creates a new RebootState
func NewRebootState() *RebootState {
	state := &RebootState{
		BaseState: BaseState{id: UpdateHubStateReboot},
	}

	return state
}
