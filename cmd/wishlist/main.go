package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/keygen"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	lm "github.com/charmbracelet/wish/logging"
	"github.com/charmbracelet/wishlist"
	"github.com/charmbracelet/wishlist/home"
	"github.com/charmbracelet/wishlist/sshconfig"
	"github.com/gliderlabs/ssh"
	"github.com/hashicorp/go-multierror"
	"gopkg.in/yaml.v3"
)

var (
	// CommitSHA is set by goreleaser on build time.
	CommitSHA = "<none>"

	// Version is set by goreleaser on build time.
	Version = "devel"
)

func main() {
	version := flag.Bool("version", false, "print version and exit")
	file := flag.String("config", "", "path to config file, can be either yaml or SSH")
	local := flag.Bool("local", false, "do not start a server, go straight into the UI")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Wishlist, a SSH directory.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *version {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Sum != "" {
			Version = info.Main.Version
		}
		fmt.Printf("wishlist version %s (%s)\n", Version, CommitSHA)
		return
	}

	config, err := getConfig(*file)
	if err != nil {
		log.Fatalln(err)
	}

	if *local {
		if err := workLocally(config); err != nil {
			log.Fatalln(err)
		}
		return
	}

	k, err := keygen.New(".wishlist", "server", nil, keygen.Ed25519)
	if err != nil {
		log.Fatalln(err)
	}
	if !k.IsKeyPairExists() {
		if err := k.WriteKeys(); err != nil {
			log.Fatalln(err)
		}
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
			s, _ := home.ExpandPath("~/.ssh/config")
			return s
		},
		func() string { return "/etc/ssh/ssh_config" },
	} {
		path := fn()
		if path == "" {
			continue
		}

		var cfg wishlist.Config
		var err error
		switch filepath.Ext(path) {
		case ".yaml", ".yml":
			cfg, err = getYAMLConfig(path)
		default:
			cfg, err = getSSHConfig(path)
		}
		if err == nil {
			log.Println("Using config from", path)
			return cfg, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			allErrs = multierror.Append(allErrs, fmt.Errorf("%q: %w", path, err))
			continue
		}
		return cfg, err
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

// nolint: wrapcheck
func workLocally(config wishlist.Config) error {
	f, err := tea.LogToFile("wishlist.log", "")
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Println(err)
		}
	}()
	m := wishlist.LocalListing(config.Endpoints)
	if err := tea.NewProgram(m).Start(); err != nil {
		return err
	}

	if m.HandoffTo() == nil {
		return nil
	}

	log.SetOutput(os.Stderr)
	return wishlist.NewLocalSSHClient().Connect(m.HandoffTo())
}
