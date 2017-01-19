package plugins

var (
	plugins = make(map[string]Plugin)
)

type Plugin struct {
	Mode              string
	CheckRequirements func() error
	Instantiate       func() interface{}
}

func RegisterPlugin(name string, plugin Plugin) {
	plugins[name] = plugin
}

func GetPlugin(mode string) interface{} {
	return plugins[mode].Instantiate()
}
