package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/charmbracelet/keygen"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	lm "github.com/charmbracelet/wish/logging"
	"github.com/charmbracelet/wishlist"
	"github.com/charmbracelet/wishlist/sshconfig"
	"github.com/gliderlabs/ssh"
	"github.com/hashicorp/go-multierror"
	"gopkg.in/yaml.v3"
)

func main() {
	file := flag.String("config", "", "path to config file, can be either yaml or SSH")
	flag.Parse()

	k, err := keygen.New(".wishlist", "server", nil, keygen.Ed25519)
	if err != nil {
		log.Fatalln(err)
	}
	if !k.IsKeyPairExists() {
		if err := k.WriteKeys(); err != nil {
			log.Fatalln(err)
		}
	}

	config, err := getConfig(*file)
	if err != nil {
		log.Fatalln(err)
	}

	config.Factory = func(e wishlist.Endpoint) (*ssh.Server, error) {
		// nolint:wrapcheck
		return wish.NewServer(
			wish.WithAddress(e.Address),
			wish.WithHostKeyPath(filepath.Join(k.KeyDir, "server_ed25519")),
			wish.WithMiddleware(
				append(
					e.Middlewares,
					lm.Middleware(),
					activeterm.Middleware(),
				)...,
			),
		)
	}

	if err := wishlist.Serve(&config); err != nil {
		log.Fatalln(err)
	}
}

func getConfig(path string) (wishlist.Config, error) {
	var allErrs error
	for _, fn := range []func() string{
		func() string { return path },
		func() string { return ".wishlist/config.yaml" },
		func() string { return ".wishlist/config.yml" },
		func() string { return ".wishlist/config" },
		func() string {
			home, err := os.UserHomeDir()
			if err != nil {
				return ""
			}
			return filepath.Join(home, ".ssh/config")
		},
		func() string { return "/etc/ssh/ssh_config" },
	} {
		path := fn()
		if path == "" {
			continue
		}

		var cfg wishlist.Config
		var err error
		ext := filepath.Ext(path)
		if ext == ".yaml" || ext == ".yml" {
			cfg, err = getYAMLConfig(path)
		} else {
			cfg, err = getSSHConfig(path)
		}
		if err == nil {
			log.Println("Using config from", path)
			return cfg, nil
		}
		allErrs = multierror.Append(allErrs, fmt.Errorf("%q: %w", path, err))
	}
	return wishlist.Config{}, fmt.Errorf("no valid config files found: %w", allErrs)
}

func getYAMLConfig(path string) (wishlist.Config, error) {
	var config wishlist.Config

	bts, err := os.ReadFile(path)
	if err != nil {
		return config, fmt.Errorf("failed to read config: %w", err)
	}

	if err := yaml.Unmarshal(bts, &config); err != nil {
		return config, fmt.Errorf("failed to parse config: %w", err)
	}

	return config, nil
}

func getSSHConfig(path string) (wishlist.Config, error) {
	config := wishlist.Config{}
	endpoints, err := sshconfig.ParseFile(path)
	if err != nil {
		return config, err // nolint:wrapcheck
	}
	config.Endpoints = endpoints
	return config, nil
}
