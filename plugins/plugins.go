package plugins

var (
	Plugins = make(map[string]Plugin)
)

type Plugin struct {
	Mode              string
	CheckRequirements func() error
	Instantiate       func() interface{}
}

func RegisterPlugin(name string, plugin Plugin) {
	Plugins[name] = plugin
}

func GetPlugin(mode string) interface{} {
	return Plugins[mode].Instantiate()
}

func CheckRequirements() error {
	for _, p := range Plugins {
		if err := p.CheckRequirements(); err != nil {
			return err
		}
	}

	return nil
}
