package handlers

type InstallUpdateHandler interface {
	Setup() error
	Install() error
	Cleanup() error
}
