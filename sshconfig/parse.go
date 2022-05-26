// Package sshconfig can parse a SSH config file into a list of endpoints.
package sshconfig

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/wishlist"
	"github.com/charmbracelet/wishlist/home"
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

	endpoints := make([]*wishlist.Endpoint, 0, infos.length())
	if err := hosts.forEach(func(name string, info hostinfo, err error) error {
		if err != nil {
			return err
		}
		if err := wildcards.forEach(func(k string, v hostinfo, err error) error {
			if err != nil {
				return err
			}
			g, err := glob.Compile(k)
			if err != nil {
				return fmt.Errorf("invalid Host: %q: %w", k, err)
			}
			if g.Match(name) {
				info = mergeHostinfo(info, v)
			}
			return nil
		}); err != nil {
			return err
		}

		endpoints = append(endpoints, &wishlist.Endpoint{
			Name: name,
			Address: net.JoinHostPort(
				firstNonEmpty(info.Hostname, name),
				firstNonEmpty(info.Port, "22"),
			),
			User:          info.User,
			IdentityFiles: info.IdentityFiles,
			ForwardAgent:  stringToBool(info.ForwardAgent),
			RequestTTY:    stringToBool(info.RequestTTY),
			RemoteCommand: info.RemoteCommand,
			SetEnv:        info.SetEnv,
			SendEnv:       append(info.SendEnv, "LC_*", "LANG"), // append default OpenSSH SendEnv's
		})
		return nil
	}); err != nil {
		return nil, err
	}

	return endpoints, nil
}

func stringToBool(s string) bool {
	ss := strings.ToLower(strings.TrimSpace(s))
	return ss == "true" || ss == "yes"
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

type hostinfo struct {
	User          string
	Hostname      string
	Port          string
	IdentityFiles []string
	ForwardAgent  string
	RequestTTY    string
	RemoteCommand string
	SendEnv       []string
	SetEnv        []string
}

type hostinfoMap struct {
	inner map[string]hostinfo
	keys  []string
	lock  sync.Mutex
}

func (m *hostinfoMap) length() int {
	m.lock.Lock()
	defer m.lock.Unlock()
	return len(m.keys)
}

func (m *hostinfoMap) set(k string, v hostinfo) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.inner[k]; !ok {
		m.keys = append(m.keys, k)
	}
	m.inner[k] = v
}

func (m *hostinfoMap) get(k string) (hostinfo, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	v, ok := m.inner[k]
	return v, ok
}

func (m *hostinfoMap) forEach(fn func(string, hostinfo, error) error) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	var err error
	for _, k := range m.keys {
		err = fn(k, m.inner[k], err)
	}
	return err
}

func newHostinfoMap() *hostinfoMap {
	return &hostinfoMap{
		inner: map[string]hostinfo{},
	}
}

func parseInternal(r io.Reader) (*hostinfoMap, error) {
	bts, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	var rb bytes.Buffer
	for _, line := range bytes.Split(bts, []byte("\n")) {
		if bytes.HasPrefix(bytes.TrimSpace(bytes.ToLower(line)), []byte("match")) {
			continue
		}
		if _, err := rb.Write(line); err != nil {
			return nil, fmt.Errorf("failed to parse: %w", err)
		}
		if _, err := rb.Write([]byte("\n")); err != nil {
			return nil, fmt.Errorf("failed to parse: %w", err)
		}
	}

	config, err := ssh_config.Decode(&rb)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	infos := newHostinfoMap()

	for _, h := range config.Hosts {
		for _, pattern := range h.Patterns {
			name := pattern.String()
			info, _ := infos.get(name)
			for _, n := range h.Nodes {
				node := strings.TrimSpace(n.String())
				if node == "" {
					continue // ignore empty nodes
				}

				if strings.HasPrefix(node, "#") {
					continue
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
					info.IdentityFiles = append(info.IdentityFiles, value)
				case "ForwardAgent":
					info.ForwardAgent = value
				case "RequestTTY":
					info.RequestTTY = value
				case "RemoteCommand":
					info.RemoteCommand = value
				case "SendEnv":
					info.SendEnv = append(info.SendEnv, value)
				case "SetEnv":
					info.SetEnv = append(info.SetEnv, value)
				case "Include":
					path, err := home.ExpandPath(value)
					if err != nil {
						return nil, err // nolint: wrapcheck
					}
					included, err := parseFileInternal(path)
					if err != nil {
						return nil, err
					}
					infos.set(name, info)
					infos = merge(infos, included)
					info, _ = infos.get(name)
				}
			}

			infos.set(name, info)
		}
	}

	return infos, nil
}

func split(m *hostinfoMap) (*hostinfoMap, *hostinfoMap) {
	wildcards := newHostinfoMap()
	hosts := newHostinfoMap()
	_ = m.forEach(func(k string, v hostinfo, _ error) error {
		// FWIW the lib always returns at least one * section... no idea why.
		if strings.Contains(k, "*") {
			wildcards.set(k, v)
			return nil
		}
		hosts.set(k, v)
		return nil
	})
	return wildcards, hosts
}

func merge(m1, m2 *hostinfoMap) *hostinfoMap {
	result := newHostinfoMap()

	_ = m1.forEach(func(k string, v hostinfo, _ error) error {
		vv, ok := m2.get(k)
		if !ok {
			result.set(k, v)
			return nil
		}
		result.set(k, mergeHostinfo(v, vv))
		return nil
	})

	_ = m2.forEach(func(k string, v hostinfo, _ error) error {
		if _, ok := m1.get(k); !ok {
			result.set(k, v)
		}
		return nil
	})
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
	h2.IdentityFiles = append(h2.IdentityFiles, h1.IdentityFiles...)
	if h1.ForwardAgent != "" {
		h2.ForwardAgent = h1.ForwardAgent
	}
	if h1.RequestTTY != "" {
		h2.RequestTTY = h1.RequestTTY
	}
	if h1.RemoteCommand != "" {
		h2.RemoteCommand = h1.RemoteCommand
	}
	h2.SendEnv = append(h2.SendEnv, h1.SendEnv...)
	h2.SetEnv = append(h2.SetEnv, h1.SetEnv...)
	return h2
}

func parseFileInternal(path string) (*hostinfoMap, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config: %w", err)
	}
	defer f.Close() // nolint:errcheck
	return parseInternal(f)
}
