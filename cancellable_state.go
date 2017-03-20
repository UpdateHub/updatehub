/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

type CancellableState struct {
	BaseState
	cancel chan bool
}

func (cs *CancellableState) Cancel(ok bool) bool {
	cs.cancel <- ok
	return ok
}

func (cs *CancellableState) Wait() {
	<-cs.cancel
}

func (cs *CancellableState) Stop() {
	close(cs.cancel)
}
