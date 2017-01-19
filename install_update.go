package main

import "bitbucket.org/ossystems/agent/handlers"

func InstallUpdate(h handlers.InstallUpdateHandler) error {
	if err := h.CheckRequirements(); err != nil {
		return err
	}

	if err := h.Setup(); err != nil {
		return err
	}

	if err := h.Install(); err != nil {
		return err
	}

	if err := h.Cleanup(); err != nil {
		return err
	}

	return nil
}
