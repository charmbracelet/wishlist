package main

import (
	"errors"
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
	"github.com/muesli/coral"
	mcoral "github.com/muesli/mango-coral"
	"github.com/muesli/roff"
	"gopkg.in/yaml.v3"
)

var (
	// CommitSHA is set by goreleaser on build time.
	CommitSHA = "<none>"

	// Version is set by goreleaser on build time.
	Version = "devel"
)

var rootCmd = &coral.Command{
	Use:   "wishlist",
	Short: "The SSH Directory",
	Long: `Wishlist is a SSH directory.

It provides a TUI for your ~/.ssh/config or another config file, which can
be in either the SSH configuration format or YAML.

It's also possible to serve the TUI over SSH using the server command.
`,
	Version:      Version,
	SilenceUsage: true,
	CompletionOptions: coral.CompletionOptions{
		HiddenDefaultCmd: true,
	},
	RunE: func(cmd *coral.Command, args []string) error {
		config, err := getConfig(configFile)
		if err != nil {
			return err
		}
		return workLocally(config)
	},
}

var manCmd = &coral.Command{
	Use:          "man",
	Args:         coral.NoArgs,
	Short:        "generate man pages",
	Hidden:       true,
	SilenceUsage: true,
	RunE: func(cmd *coral.Command, args []string) error {
		manPage, err := mcoral.NewManPage(1, rootCmd)
		if err != nil {
			return err
		}
		manPage = manPage.WithSection("Copyright", "(C) 2022 Charmbracelet, Inc.\n"+
			"Released under MIT license.")
		fmt.Println(manPage.Build(roff.NewDocument()))
		return nil
	},
}

var serverCmd = &coral.Command{
	Use:     "serve",
	Aliases: []string{"server", "s"},
	Args:    coral.NoArgs,
	Short:   "Serve the TUI over SSH.",
	RunE: func(cmd *coral.Command, args []string) error {
		config, err := getConfig(configFile)
		if err != nil {
			return err
		}
		k, err := keygen.New(".wishlist", "server", nil, keygen.Ed25519)
		if err != nil {
			return err
		}
		if !k.IsKeyPairExists() {
			if err := k.WriteKeys(); err != nil {
				return err
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

		return wishlist.Serve(&config)
	},
}

var configFile string

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to the config file to use. Defaults to, in order of preference: $PWD/.wishlist/config.yaml, $PWD/.wishlist/config.yml, $HOME/.ssh/config, /etc/ssh/ssh_config")
	rootCmd.AddCommand(serverCmd, manCmd)
}

func main() {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Sum != "" {
		Version = fmt.Sprintf("%s (%s)", info.Main.Version, CommitSHA)
	}
	if err := rootCmd.Execute(); err != nil {
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
	m := wishlist.NewListing(config.Endpoints, nil)
	if err := tea.NewProgram(m).Start(); err != nil {
		return err
	}

	if m.HandoffTo() == nil {
		return nil
	}

	log.SetOutput(os.Stderr)
	return wishlist.NewLocalSSHClient().Connect(m.HandoffTo())
}
