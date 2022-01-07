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
	"gopkg.in/yaml.v3"
)

func main() {
	file := flag.String("config", ".wishlist/config.yaml", "path to config file")
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
		return wish.NewServer(
			wish.WithAddress(e.Address),
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
	ext := filepath.Ext(path)
	if ext == ".yaml" || ext == ".yml" {
		return getYAMLConfig(path)
	}
	return getSSHConfig(path)
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

	log.Println("Using config from", path)
	return config, nil
}

func getSSHConfig(path string) (wishlist.Config, error) {
	for _, fn := range []func() string{
		func() string { return path },
		func() string { return ".wishlist/config" },
		func() string { return "/etc/ssh/ssh_config" },
		func() string {
			home, err := os.UserHomeDir()
			if err != nil {
				return ""
			}
			return filepath.Join(home, ".ssh/config")
		},
	} {
		path := fn()
		cfg, err := getSSHConfigFrom(path)
		if err == nil {
			log.Println("Using config from", path)
			return cfg, nil
		}
	}
	return wishlist.Config{}, fmt.Errorf("no ssh config files found")
}

func getSSHConfigFrom(path string) (wishlist.Config, error) {
	config := wishlist.Config{}
	endpoints, err := sshconfig.ParseFile(path)
	if err != nil {
		return config, err
	}
	config.Endpoints = endpoints
	return config, nil
}
