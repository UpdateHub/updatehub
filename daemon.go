/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import "github.com/Sirupsen/logrus"

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
		err := d.uh.ReportCurrentState()
		if err != nil {
			d.uh.logger.WithFields(logrus.Fields{
				"state": StateToString(d.uh.state.ID()),
			}).Warn("Failed to report status")
		}

		state, _ := d.uh.state.Handle(d.uh)

		if state.ID() == UpdateHubStateError {
			if es, ok := state.(*ErrorState); ok {
				d.uh.logger.Warn(es.cause)
			}
		}

		d.uh.state = state

		if d.stop {
			return
		}
	}
}
