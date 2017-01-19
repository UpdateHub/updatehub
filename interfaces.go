package main

type PackageObjectInstaller interface {
	CheckRequirements() error
	Setup() error
	Install() error
	Cleanup() error
}
