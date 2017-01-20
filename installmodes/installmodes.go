package installmodes

import "errors"

var (
	InstallModes = make(map[string]InstallMode)
)

type InstallMode struct {
	Mode              string
	CheckRequirements func() error
	Instantiate       func() interface{}
}

func RegisterInstallMode(name string, mode InstallMode) {
	InstallModes[name] = mode
}

func GetObject(name string) (interface{}, error) {
	if m, ok := InstallModes[name]; ok {
		return m.Instantiate(), nil
	} else {
		return nil, errors.New("Object not found")
	}
}

func CheckRequirements() error {
	for _, m := range InstallModes {
		if err := m.CheckRequirements(); err != nil {
			return err
		}
	}

	return nil
}
