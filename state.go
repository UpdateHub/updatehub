package main

type EasyFotaState int

const (
	EasyFotaStateIdle = iota
	EasyFotaStateUpdateCheck
	EasyFotaStateUpdateFetch
	EasyFotaStateInstalling
	EasyFotaStateInstalled
	EasyFotaStateWaitingForReboot
)

var statusNames = map[EasyFotaState]string{
	EasyFotaStateIdle:             "idle",
	EasyFotaStateUpdateCheck:      "update-check",
	EasyFotaStateUpdateFetch:      "update-fetch",
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

func (b *BaseState) Cancel(ok bool) bool {
	return ok
}

type State interface {
	Id() EasyFotaState
	Handle(*EasyFota) (State, bool)
	Cancel(bool) bool
}

func StateToString(status EasyFotaState) string {
	return statusNames[status]
}
