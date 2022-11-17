package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/keygen"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	lm "github.com/charmbracelet/wish/logging"
	"github.com/charmbracelet/wishlist"
	"github.com/charmbracelet/wishlist/sshconfig"
	"github.com/hashicorp/go-multierror"
	mcobra "github.com/muesli/mango-cobra"
	"github.com/muesli/roff"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	// CommitSHA is set by goreleaser on build time.
	CommitSHA = "<none>"

	// Version is set by goreleaser on build time.
	Version = "devel"
)

var rootCmd = &cobra.Command{
	Use:   "wishlist",
	Short: "The SSH Directory",
	Long: `Wishlist is a SSH directory.

It provides a TUI for your ~/.ssh/config or another config file, which can
be in either the SSH configuration format or YAML.

It's also possible to serve the TUI over SSH using the server command.
`,
	Version:      Version,
	SilenceUsage: true,
	Args:         cobra.MaximumNArgs(1),
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := getConfig(configFile)
		if err != nil {
			return err
		}
		return workLocally(config, args)
	},
}

var manCmd = &cobra.Command{
	Use:          "man",
	Args:         cobra.NoArgs,
	Short:        "generate man pages",
	Hidden:       true,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		manPage, err := mcobra.NewManPage(1, rootCmd)
		if err != nil {
			return fmt.Errorf("could not generate man pages: %w", err)
		}
		manPage = manPage.WithSection("Copyright", "(C) 2022 Charmbracelet, Inc.\n"+
			"Released under MIT license.")
		fmt.Println(manPage.Build(roff.NewDocument()))
		return nil
	},
}

var serverCmd = &cobra.Command{
	Use:     "serve",
	Aliases: []string{"server", "s"},
	Args:    cobra.NoArgs,
	Short:   "Serve the TUI over SSH.",
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := getConfig(configFile)
		if err != nil {
			return err
		}
		k, err := keygen.New(".wishlist/server", nil, keygen.Ed25519)
		if err != nil {
			return fmt.Errorf("could not create keypair: %w", err)
		}
		if !k.KeyPairExists() {
			if err := k.WriteKeys(); err != nil {
				return fmt.Errorf("could not write key pair: %w", err)
			}
		}

		config.Factory = func(e wishlist.Endpoint) (*wish.Server, error) {
			// nolint:wrapcheck
			return wish.NewServer(
				wish.WithAddress(e.Address),
				wish.WithHostKeyPath(".wishlist/server_ed25519"),
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
			return fmt.Errorf("could not serve wishlist: %w", err)
		}
		return nil
	},
}

var configFile string

func init() {
	paths := userConfigPaths()
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to the config file to use. Defaults to, in order of preference: "+strings.Join(paths, ", "))
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

func userConfigPaths() []string {
	paths := []string{
		".wishlist/config.yaml",
		".wishlist/config.yml",
		".wishlist/config",
	}

	if cfg, err := os.UserConfigDir(); err == nil {
		paths = append(
			paths,
			filepath.Join(cfg, "wishlist.yaml"),
			filepath.Join(cfg, "wishlist.yml"),
			filepath.Join(cfg, "wishlist"),
		)
	}

	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".ssh", "config"))
	}

	return append(paths, "/etc/ssh/ssh_config")
}

func getConfig(configFile string) (wishlist.Config, error) {
	var allErrs error
	for _, path := range append([]string{configFile}, userConfigPaths()...) {
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
		if err != nil {
			log.Println("Not using", path, ":", err)
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
func workLocally(config wishlist.Config, args []string) error {
	f, err := tea.LogToFile("wishlist.log", "")
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Println(err)
		}
	}()

	// either no args or arg is a list
	if len(args) == 0 || args[0] == "list" {
		m := wishlist.NewListing(config.Endpoints, wishlist.NewLocalSSHClient())
		_, err := tea.NewProgram(m).Run()
		return err
	}

	// ssh directly into something by its name
	for _, e := range config.Endpoints {
		if e.Name == args[0] {
			return connect(e)
		}
	}

	return fmt.Errorf("invalid endpoint name: %q", args[0])
}

func connect(e *wishlist.Endpoint) error {
	cmd := wishlist.NewLocalSSHClient().For(e)
	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stderr)
	cmd.SetStdin(os.Stdin)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	return nil
}
