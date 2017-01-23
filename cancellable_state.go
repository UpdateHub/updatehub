package main

type CancellableState struct {
	BaseState
	cancel chan bool
}

func (cs *CancellableState) Cancel() bool {
	cs.cancel <- true
	return true
}

func (cs *CancellableState) Wait() {
	<-cs.cancel
}

func (cs *CancellableState) Stop() {
	close(cs.cancel)
}
