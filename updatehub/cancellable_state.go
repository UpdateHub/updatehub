/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import "sync"

type CancellableState struct {
	BaseState
	cancel         chan bool
	nextState      State
	nextStateMutex sync.Mutex
}

func (cs *CancellableState) NextState() State {
	cs.nextStateMutex.Lock()
	defer cs.nextStateMutex.Unlock()

	return cs.nextState
}

func (cs *CancellableState) Cancel(ok bool, nextState State) bool {
	cs.nextStateMutex.Lock()
	defer cs.nextStateMutex.Unlock()

	cs.cancel <- ok
	cs.nextState = nextState
	return ok
}

func (cs *CancellableState) Wait() {
	<-cs.cancel
}

func (cs *CancellableState) Stop() {
	close(cs.cancel)
}
