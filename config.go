package wishlist

import (
	"fmt"

	"github.com/charmbracelet/wish"
	"github.com/gliderlabs/ssh"
)

// Endpoint represents an endpoint to list.
// If it has a Handler, wishlist will start an SSH server on the given address.
type Endpoint struct {
	Name        string            `yaml:"name"`    // Endpoint name.
	Address     string            `yaml:"address"` // Endpoint address in the `host:port` format, if empty, will be the same address as the list, increasing the port number.
	User        string            `yaml:"user"`    // User to authenticate as.
	Middlewares []wish.Middleware `yaml:"-"`       // wish middlewares you can use in the factory method.
}

// String returns the endpoint in a friendly string format.
func (e *Endpoint) String() string {
	return fmt.Sprintf(`%q => "%s@%s"`, e.Name, e.User, e.Address)
}

// Valid returns true if the endpoint is valid.
func (e Endpoint) Valid() bool {
	return e.Name != "" && (len(e.Middlewares) > 0 || e.Address != "")
}

// ShouldListen returns true if we should start a server for this endpoint.
func (e Endpoint) ShouldListen() bool {
	return len(e.Middlewares) > 0
}

// Config represents the wishlist configuration.
type Config struct {
	Listen    string                              `yaml:"listen"`    // Address to listen on.
	Port      int64                               `yaml:"port"`      // Port to start the first server on.
	Endpoints []*Endpoint                         `yaml:"endpoints"` // Endpoints to list.
	Factory   func(Endpoint) (*ssh.Server, error) `yaml:"-"`         // Factory used to create the SSH server for the given endpoint.
	Users     []User                              `yaml:"users"`     // Users allowed to access the list.

	lastPort int64
}

// User contains user-level configuration for a repository.
type User struct {
	Name       string   `yaml:"name"`
	PublicKeys []string `yaml:"public-keys"`
}
