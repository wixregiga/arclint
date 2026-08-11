package shared

type registryFactory struct{}

// RegistryFactory registers features by name.
var RegistryFactory registryFactory

// Register records a feature under its name.
func (registryFactory) Register(name string) {}

func init() {
	RegistryFactory.Register("alpha")
	RegistryFactory.Register("beta")
}
