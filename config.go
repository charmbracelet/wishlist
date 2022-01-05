package wishlist

import (
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/gliderlabs/ssh"
)

// Endpoint represents an endpoint to list.
// If it has a Handler, wishlist will start an SSH server on the given address.
type Endpoint struct {
	Name    string                     `yaml:"name"`
	Address string                     `yaml:"address"`
	User    string                     `yaml:"user"`
	Handler bubbletea.BubbleTeaHandler `yaml:"-"`
}

// Returns true if the endpoint is valid.
func (e Endpoint) Valid() bool {
	return e.Name != "" && (e.Handler != nil || e.Address != "")
}

// ShouldListen returns true if we should start a server for this endpoint.
func (e Endpoint) ShouldListen() bool {
	return e.Handler != nil
}

// Config represents the wishlist configuration.
type Config struct {
	Listen    string                              `yaml:"listen"`
	Port      int64                               `yaml:"port"`
	Endpoints []*Endpoint                         `yaml:"endpoints"`
	Factory   func(Endpoint) (*ssh.Server, error) `yaml:"-"`

	lastPort int64
}
