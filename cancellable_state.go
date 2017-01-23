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
