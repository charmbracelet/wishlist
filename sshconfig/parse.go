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
	"github.com/gobwas/glob"
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
	infos, err := parseInternal(r)
	if err != nil {
		return nil, err
	}

	wildcards, hosts := split(infos)

	endpoints := make([]*wishlist.Endpoint, 0, len(infos))
	for name, info := range hosts {
		for k, v := range wildcards {
			g, err := glob.Compile(k)
			if err != nil {
				return nil, err
			}
			if g.Match(name) {
				info = mergeHostinfo(info, v)
			}
		}

		if info.Hostname == "" {
			info.Hostname = name // Host foo.bar, use foo.bar as name and HostName
		}
		if info.Port == "" {
			info.Port = "22"
		}

		endpoints = append(endpoints, &wishlist.Endpoint{
			Name:         name,
			Address:      net.JoinHostPort(info.Hostname, info.Port),
			User:         info.User,
			IdentityFile: info.IdentityFile,
		})
	}

	return endpoints, nil
}

type hostinfo struct {
	User         string
	Hostname     string
	Port         string
	IdentityFile string
}

func parseInternal(r io.Reader) (map[string]hostinfo, error) {
	config, err := ssh_config.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	infos := map[string]hostinfo{}

	for _, h := range config.Hosts {
		for _, pattern := range h.Patterns {
			name := pattern.String()
			info := infos[name]
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
				case "IdentityFile":
					info.IdentityFile = value
				case "Include":
					included, err := parseFileInternal(value)
					if err != nil {
						return nil, err
					}
					log.Println("INCLUDED", included)
					infos[name] = info
					infos = merge(infos, included)
					info = infos[name]
				}
			}

			infos[name] = info
		}
	}

	return infos, nil
}

func split(m map[string]hostinfo) (wildcards map[string]hostinfo, hosts map[string]hostinfo) {
	wildcards = map[string]hostinfo{}
	hosts = map[string]hostinfo{}
	for k, v := range m {
		if strings.Contains(k, "*") {
			wildcards[k] = v
			continue
		}
		hosts[k] = v
	}
	return
}

func merge(m1, m2 map[string]hostinfo) map[string]hostinfo {
	result := map[string]hostinfo{}

	for k, v := range m1 {
		vv, ok := m2[k]
		if !ok {
			result[k] = v
			continue
		}
		result[k] = mergeHostinfo(v, vv)
	}

	for k, v := range m2 {
		if _, ok := m1[k]; !ok {
			result[k] = v
		}
	}
	return result
}

func mergeHostinfo(h1, h2 hostinfo) hostinfo {
	if h1.Port != "" {
		h2.Port = h1.Port
	}
	if h1.Hostname != "" {
		h2.Hostname = h1.Hostname
	}
	if h1.User != "" {
		h2.User = h1.User
	}
	if h1.IdentityFile != "" {
		h2.IdentityFile = h1.IdentityFile
	}
	return h2
}

func parseFileInternal(path string) (map[string]hostinfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config: %w", err)
	}
	defer f.Close() // nolint:errcheck
	return parseInternal(f)
}
