package port

import "context"

// Factory constructs a gateway instance. Concrete type must implement the PortKind interface.
type Factory func(cfg map[string]string) (any, error)

// Registration is one gateway in the process registry.
type Registration struct {
	ID           GatewayID
	Port         PortKind
	Capabilities Capability
	Factory      Factory
	Probe        func(ctx context.Context) Health
	Instance     any // optional pre-built instance (fakes in tests)
}

// Registry stores gateway registrations.
type Registry interface {
	Register(r Registration) error
	Get(id GatewayID) (Registration, bool)
	List(port PortKind) []Registration
}
