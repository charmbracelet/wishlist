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

func ParseFile(path string) ([]*wishlist.Endpoint, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config: %w", err)
	}
	defer f.Close()
	return ParseReader(f)
}

func ParseReader(r io.Reader) ([]*wishlist.Endpoint, error) {
	config, err := ssh_config.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var endpoints []*wishlist.Endpoint

	for _, h := range config.Hosts {
		name := h.Patterns[0].String()

		if strings.Contains(name, "*") {
			// ignore wildcards
			continue
		}

		var host string
		var port string
		var user string
		for _, n := range h.Nodes {
			node := strings.TrimSpace(n.String())
			if node == "" {
				// ignore empty nodes
				continue
			}
			parts := strings.SplitN(node, " ", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid node on app %q: %q", name, node)
			}
			switch parts[0] {
			case "HostName":
				host = parts[1]
			case "User":
				user = parts[1]
			case "Port":
				port = parts[1]
			default:
				log.Printf("ignoring invalid node type %q on host %q", parts[0], h.Patterns[0].String())
			}
		}

		if port == "" {
			port = "22"
		}

		endpoints = append(endpoints, &wishlist.Endpoint{
			Name:    name,
			Address: net.JoinHostPort(host, port),
			User:    user,
		})
	}

	return endpoints, nil
}
