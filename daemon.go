/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import "fmt"

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

func (d *Daemon) Run() {
	for {
		d.uh.ReportCurrentState()

		state, _ := d.uh.state.Handle(d.uh)

		if state.ID() == UpdateHubStateError {
			if es, ok := state.(*ErrorState); ok {
				// FIXME: log error
				fmt.Println(es.cause)
			}
		}

		d.uh.state = state

		if d.stop {
			return
		}
	}
}
