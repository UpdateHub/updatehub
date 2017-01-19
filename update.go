package main

type InstallUpdateHandler interface {
	CheckRequirements() error
	Setup() error
}

func InstallUpdate(h InstallUpdateHandler) {
	h.CheckRequirements()
	h.Setup()
}
