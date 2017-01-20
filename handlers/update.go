package handlers

type InstallUpdateHandler interface {
	CheckRequirements() error
	Setup() error
	Install() error
	Cleanup() error
}
