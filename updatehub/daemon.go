/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package updatehub

type Daemon struct {
	uh   *UpdateHub
	stop bool
}

func NewDaemon(uh *UpdateHub) *Daemon {
	return &Daemon{
		uh: uh,
	}
}

func (d *Daemon) Stop() {
	d.stop = true
}

func (d *Daemon) Run() int {
	for {
		nextState := d.uh.ProcessCurrentState()

		d.uh.SetState(nextState)

		if d.stop || nextState.ID() == UpdateHubStateExit {
			if finalState, _ := nextState.(*ExitState); finalState != nil {
				return finalState.exitCode
			}

			return 0
		}
	}
}
