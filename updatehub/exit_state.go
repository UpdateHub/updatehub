/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package updatehub

// ExitState is the final state of the state machine
type ExitState struct {
	BaseState

	exitCode int
}

// NewExitState creates a new ExitState
func NewExitState(exitCode int) *ExitState {
	return &ExitState{
		BaseState: BaseState{id: UpdateHubStateExit},
		exitCode:  exitCode,
	}
}

// Handle for ExitState
func (state *ExitState) Handle(uh *UpdateHub) (State, bool) {
	panic("ExitState handler should not be called")
}
