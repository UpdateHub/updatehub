package main

type CheckRequirements interface {
	CheckRequirements() error
}

type Setup interface {
	Setup() error
}
