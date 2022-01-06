package wishlist

import (
	"github.com/charmbracelet/wish"
	"github.com/gliderlabs/ssh"
)

// Endpoint represents an endpoint to list.
// If it has a Handler, wishlist will start an SSH server on the given address.
type Endpoint struct {
	Name        string            `yaml:"name"`
	Address     string            `yaml:"address"`
	User        string            `yaml:"user"`
	Middlewares []wish.Middleware `yaml:"-"`
}

// Returns true if the endpoint is valid.
func (e Endpoint) Valid() bool {
	return e.Name != "" && (len(e.Middlewares) > 0 || e.Address != "")
}

// ShouldListen returns true if we should start a server for this endpoint.
func (e Endpoint) ShouldListen() bool {
	return len(e.Middlewares) > 0
}

// Config represents the wishlist configuration.
type Config struct {
	Listen    string                              `yaml:"listen"`
	Port      int64                               `yaml:"port"`
	Endpoints []*Endpoint                         `yaml:"endpoints"`
	Factory   func(Endpoint) (*ssh.Server, error) `yaml:"-"`

	lastPort int64
}
