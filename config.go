package wishlist

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/gobwas/glob"
)

const (
	authModePassword            = "password"
	authModePublicKey           = "publickey"
	authModeKeyboardInteractive = "keyboard-interactive"
)

// Link defines an item link.
type Link struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

func (l Link) String() string {
	if l.URL == "" {
		return ""
	}

	if l.Name == "" {
		return l.URL
	}

	return fmt.Sprintf("%s %s", l.Name, l.URL)
}

// Endpoint represents an endpoint to list.
// If it has a Handler, wishlist will start an SSH server on the given address.
type Endpoint struct {
	Name                     string            `yaml:"name"`                      // Endpoint name.
	Address                  string            `yaml:"address"`                   // Endpoint address in the `host:port` format, if empty, will be the same address as the list, increasing the port number.
	User                     string            `yaml:"user"`                      // User to authenticate as.
	ForwardAgent             bool              `yaml:"forward_agent"`             // ForwardAgent defines whether to forward the current agent. Anologous to SSH's config ForwardAgent.
	RequestTTY               bool              `yaml:"request_tty"`               // RequestTTY defines whether to request a TTY. Anologous to SSH's config RequestTTY.
	RemoteCommand            string            `yaml:"remote_command"`            // RemoteCommand defines whether to request a TTY. Anologous to SSH's config RemoteCommand.
	Desc                     string            `yaml:"description"`               // Description describes an optional description of the item.
	Link                     Link              `yaml:"link"`                      // Links can be used to add a link to the item description using OSC8.
	ProxyJump                string            `yaml:"proxy_jump"`                // Anologous to SSH's ProxyJump
	SendEnv                  []string          `yaml:"send_env"`                  // Anologous to SSH's SendEnv
	SetEnv                   []string          `yaml:"set_env"`                   // Anologous to SSH's SetEnv
	PreferredAuthentications []string          `yaml:"preferred_authentications"` // Anologous to SSH's PreferredAuthentications
	IdentityFiles            []string          `yaml:"identity_files"`            // IdentityFiles is only used when in local mode.
	Timeout                  time.Duration     `yaml:"connect_timeout"`           // Connection timeout.
	Middlewares              []wish.Middleware `yaml:"-"`                         // wish middlewares you can use in the factory method.
}

// EndpointHint can be used to match a discovered endpoint (through zeroconf
// for example) and set additional options into it.
type EndpointHint struct {
	Match                    string        `yaml:"match"`
	Port                     string        `yaml:"port"`
	User                     string        `yaml:"user"`
	ForwardAgent             *bool         `yaml:"forward_agent"`
	RequestTTY               *bool         `yaml:"request_tty"`
	RemoteCommand            string        `yaml:"remote_command"`
	Desc                     string        `yaml:"description"`
	Link                     Link          `yaml:"link"`
	ProxyJump                string        `yaml:"proxy_jump"`
	SendEnv                  []string      `yaml:"send_env"`
	SetEnv                   []string      `yaml:"set_env"`
	PreferredAuthentications []string      `yaml:"preferred_authentications"`
	IdentityFiles            []string      `yaml:"identity_files"`
	Timeout                  time.Duration `yaml:"connect_timeout"`
}

// Authentications returns either the client preferred authentications or the
// default publickey,keyboard-interactive.
func (e Endpoint) Authentications() []string {
	if len(e.PreferredAuthentications) == 0 {
		return []string{authModePublicKey, authModeKeyboardInteractive}
	}
	return e.PreferredAuthentications
}

// Environment evaluates SendEnv and SetEnv into the env map that should be
// set into the session.
// Optionally you can pass a list existing environment variables
// (e.g. os.Environ()), and the ones allowed by SendEnv will be set as well.
// As on OpenSSH, envs set via SetEnv take precedence over the ones from
// hostenv.
func (e Endpoint) Environment(hostenv ...string) map[string]string {
	env := map[string]string{}

	for _, set := range hostenv {
		k, v, ok := strings.Cut(set, "=")
		if !ok || k == "" {
			continue
		}
		if e.shouldSend(k) {
			env[k] = v
		} else {
			log.Debug("ignored", "env", k)
		}
	}

	for _, set := range e.SetEnv {
		k, v, ok := strings.Cut(set, "=")
		if !ok || k == "" {
			continue
		}
		env[k] = v
	}

	return env
}

func (e Endpoint) shouldSend(k string) bool {
	for _, send := range append(e.SendEnv, "LC_*", "LANG") { // append default OpenSSH SendEnv's
		glob, err := glob.Compile(send)
		if err != nil {
			continue
		}
		if glob.Match(k) {
			return true
		}
	}
	return false
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
	Listen       string                              `yaml:"listen"`    // Address to listen on.
	Port         int64                               `yaml:"port"`      // Port to start the first server on.
	Endpoints    []*Endpoint                         `yaml:"endpoints"` // Endpoints to list.
	Hints        []EndpointHint                      `yaml:"hints"`     // Endpoints hints to apply to discovered hosts.
	Factory      func(Endpoint) (*ssh.Server, error) `yaml:"-"`         // Factory used to create the SSH server for the given endpoint.
	Users        []User                              `yaml:"users"`     // Users allowed to access the list.
	Metrics      Metrics                             `yaml:"metrics"`   // Metrics configuration.
	EndpointChan chan []*Endpoint                    `yaml:"-"`         // Channel to update the endpoints. Used only in server mode.

	lastPort int64
}

// User contains user-level configuration for a repository.
type User struct {
	Name       string   `yaml:"name"`
	PublicKeys []string `yaml:"public-keys"`
}

// Metrics configuration.
type Metrics struct {
	Enabled bool   `yaml:"enabled"`
	Name    string `yaml:"name"`
	Address string `yaml:"address"`
}
