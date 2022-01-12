// Package sshconfig can parse a SSH config file into a list of endpoints.
package sshconfig

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"github.com/charmbracelet/wishlist"
	"github.com/kevinburke/ssh_config"
)

// PraseFile reads and parses the file in the given path.
func ParseFile(path string) ([]*wishlist.Endpoint, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config: %w", err)
	}
	defer f.Close() // nolint:errcheck
	return ParseReader(f)
}

// ParseReader reads and parses the given reader.
func ParseReader(r io.Reader) ([]*wishlist.Endpoint, error) {
	config, err := ssh_config.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	infos := map[string]hostinfo{}

	for _, h := range config.Hosts {
		for _, pattern := range h.Patterns {
			name := pattern.String()
			info := infos[name]

			if strings.Contains(name, "*") {
				continue // ignore wildcards
			}

			for _, n := range h.Nodes {
				node := strings.TrimSpace(n.String())
				if node == "" {
					continue // ignore empty nodes
				}

				parts := strings.SplitN(node, " ", 2) // nolint:gomnd
				if len(parts) != 2 {                  // nolint:gomnd
					return nil, fmt.Errorf("invalid node on app %q: %q", name, node)
				}

				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch key {
				case "HostName":
					info.Hostname = value
				case "User":
					info.User = value
				case "Port":
					info.Port = value
				default:
					log.Printf("ignoring invalid node type %q on host %q", key, name)
				}
			}

			infos[name] = info
		}
	}

	endpoints := make([]*wishlist.Endpoint, 0, len(infos))
	for name, info := range infos {
		if info.Port == "" {
			info.Port = "22"
		}
		endpoints = append(endpoints, &wishlist.Endpoint{
			Name:    name,
			Address: net.JoinHostPort(info.Hostname, info.Port),
			User:    info.User,
		})
	}

	return endpoints, nil
}

type hostinfo struct {
	User     string
	Hostname string
	Port     string
}
