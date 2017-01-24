package main

type StateTestController struct {
	EasyFota

	updateAvailable  bool
	fetchUpdateError error
}

func (c *StateTestController) CheckUpdate() bool {
	return c.updateAvailable
}

func (c *StateTestController) FetchUpdate() error {
	return c.fetchUpdateError
}
