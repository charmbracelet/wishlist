package wishlist

import (
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/gliderlabs/ssh"
)

// Endpoint represents an endpoint to list.
// If it has a Handler, wishlist will start an SSH server on the given address.
type Endpoint struct {
	Name    string                     `json:"name"`
	Address string                     `json:"address"`
	Handler bubbletea.BubbleTeaHandler `json:"-"`
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
	Listen    string                              `json:"listen"`
	Port      int64                               `json:"port"`
	Endpoints []*Endpoint                         `json:"endpoints"`
	Factory   func(Endpoint) (*ssh.Server, error) `json:"-"`

	lastPort int64
}
