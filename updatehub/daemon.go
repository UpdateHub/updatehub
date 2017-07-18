/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
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
		d.uh.ReportCurrentState()

		state, cancel := d.uh.State.Handle(d.uh)

		cs, ok := d.uh.State.(*CancellableState)
		if cancel && ok {
			d.uh.State = cs.NextState()
		} else {
			d.uh.State = state
		}

		if d.stop || state.ID() == UpdateHubStateExit {
			if finalState, _ := state.(*ExitState); finalState != nil {
				return finalState.exitCode
			}

			return 0
		}
	}
}
