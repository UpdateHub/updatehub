package main

type EasyFotaState int

const (
	EasyFotaStateIdle = iota
	EasyFotaStateUpdateCheck
	EasyFotaStateInstalling
	EasyFotaStateInstalled
	EasyFotaStateWaitingForReboot
)

var statusNames = map[EasyFotaState]string{
	EasyFotaStateIdle:             "idle",
	EasyFotaStateUpdateCheck:      "update-check",
	EasyFotaStateInstalling:       "installing",
	EasyFotaStateInstalled:        "installed",
	EasyFotaStateWaitingForReboot: "waiting-for-reboot",
}

type BaseState struct {
	id EasyFotaState
}

func (b *BaseState) Id() EasyFotaState {
	return b.id
}

func (b *BaseState) Cancel() bool {
	return false
}

type State interface {
	Id() EasyFotaState
	Handle(*EasyFota) State
	Cancel() bool
}

func StateToString(status EasyFotaState) string {
	return statusNames[status]
}
