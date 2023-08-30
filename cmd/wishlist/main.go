package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/keygen"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	lm "github.com/charmbracelet/wish/logging"
	"github.com/charmbracelet/wishlist"
	"github.com/charmbracelet/wishlist/srv"
	"github.com/charmbracelet/wishlist/sshconfig"
	"github.com/charmbracelet/wishlist/tailscale"
	"github.com/charmbracelet/wishlist/zeroconf"
	"github.com/gobwas/glob"
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
	Version:       Version,
	SilenceUsage:  true,
	SilenceErrors: false,
	Args:          cobra.MaximumNArgs(1),
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		for k, v := range map[string]string{
			"TAILSCALE_KEY":           "tailscale.key",
			"TAILSCALE_CLIENT_ID":     "tailscale.client.id",
			"TAILSCALE_CLIENT_SECRET": "tailscale.client.secret",
		} {
			if e := os.Getenv(k); e != "" {
				if err := cmd.Flags().Set(v, e); err != nil {
					return err
				}
			}
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cache, err := os.UserCacheDir()
		if err != nil {
			return fmt.Errorf("could not create log file: %w", err)
		}
		f, err := os.OpenFile(filepath.Join(cache, "wishlist.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644) //nolint:gomnd
		if err != nil {
			return err //nolint: wrapcheck
		}
		log.SetOutput(f)
		log.SetLevel(log.DebugLevel)
		log.SetFormatter(log.JSONFormatter)

		defer func() {
			if err := f.Close(); err != nil {
				log.Info("failes to close wishlist.log", "err", err)
			}
		}()

		seed, err := getSeedEndpoints(cmd.Context())
		if err != nil {
			return err
		}
		config, _, err := getConfig(configFile, seed)
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
	RunE: func(_ *cobra.Command, _ []string) error {
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
	Use:           "serve",
	Aliases:       []string{"server", "s"},
	Args:          cobra.NoArgs,
	Short:         "Serve the TUI over SSH.",
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		seed, err := getSeedEndpoints(cmd.Context())
		if err != nil {
			return err
		}
		config, path, err := getConfig(configFile, seed)
		if err != nil {
			return err
		}

		if refreshInterval > 0 {
			log.Info("endpoints", "refresh.interval", refreshInterval)
			config.EndpointChan = make(chan []*wishlist.Endpoint)
			ticker := time.NewTicker(refreshInterval)
			defer ticker.Stop()
			go func() {
				for range ticker.C {
					log.Info("refreshing endpoints...")
					ctx := context.Background()
					seed, err := getSeedEndpoints(ctx)
					if err != nil {
						log.Error("could not get seed endpoints", "error", err)
						continue
					}
					reloaded, err := getConfigFile(path, seed)
					if err != nil {
						log.Error("could not load configuration file", "error", err)
						continue
					}
					config.EndpointChan <- reloaded.Endpoints
				}
			}()
		}

		k, err := keygen.New(".wishlist/server_ed25519", keygen.WithKeyType(keygen.Ed25519))
		if err != nil {
			return fmt.Errorf("could not create keypair: %w", err)
		}
		if !k.KeyPairExists() {
			if err := k.WriteKeys(); err != nil {
				return fmt.Errorf("could not write key pair: %w", err)
			}
		}

		config.Factory = func(e wishlist.Endpoint) (*ssh.Server, error) {
			//nolint:wrapcheck
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

var (
	configFile            string
	srvDomains            []string
	refreshInterval       time.Duration
	zeroconfEnabled       bool
	zeroconfDomain        string
	zeroconfTimeout       time.Duration
	tailscaleNet          string
	tailscaleKey          string
	tailscaleClientID     string
	tailscaleClientSecret string
)

func init() {
	paths := userConfigPaths()
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to the config file to use. Defaults to, in order of preference: "+strings.Join(paths, ", "))
	serverCmd.PersistentFlags().DurationVar(&refreshInterval, "endpoints.refresh.interval", 0, "Interval to refresh the endpoints, with 0 disabling it. Defaults to 0")
	rootCmd.PersistentFlags().BoolVar(&zeroconfEnabled, "zeroconf.enabled", false, "Whether to enable zeroconf service discovery (Avahi/Bonjour/mDNS)")
	rootCmd.PersistentFlags().StringVar(&zeroconfDomain, "zeroconf.domain", "", "Domain to use with zeroconf service discovery")
	rootCmd.PersistentFlags().DurationVar(&zeroconfTimeout, "zeroconf.timeout", time.Second, "How long should zeroconf keep searching for hosts")
	rootCmd.PersistentFlags().StringSliceVar(&srvDomains, "srv.domain", nil, "SRV domains to discover endpoints")
	rootCmd.PersistentFlags().StringVar(&tailscaleNet, "tailscale.net", "", "Tailscale tailnet name")
	rootCmd.PersistentFlags().StringVar(&tailscaleKey, "tailscale.key", "", "Tailscale API key [$TAILSCALE_KEY]")
	rootCmd.PersistentFlags().StringVar(&tailscaleClientID, "tailscale.client.id", "", "Tailscale client ID [$TAILSCALE_CLIENT_ID]")
	rootCmd.PersistentFlags().StringVar(&tailscaleClientSecret, "tailscale.client.secret", "", "Tailscale client Secret [$TAILSCALE_CLIENT_SECRET]")
	rootCmd.MarkFlagsMutuallyExclusive("tailscale.key", "tailscale.client.id")
	rootCmd.MarkFlagsRequiredTogether("tailscale.client.id", "tailscale.client.secret")
	rootCmd.AddCommand(serverCmd, manCmd)
}

func main() {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Sum != "" {
		Version = fmt.Sprintf("%s (%s)", info.Main.Version, CommitSHA)
	}
	if err := rootCmd.Execute(); err != nil {
		log.Fatal("command failed", "err", err)
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

func applyHints(seed []*wishlist.Endpoint, hints []wishlist.EndpointHint) []*wishlist.Endpoint {
	for _, hint := range hints {
		glob, err := glob.Compile(hint.Match)
		if err != nil {
			log.Error("invalid hint match", "match", hint.Match, "error", err)
			continue
		}
		for i, end := range seed {
			if !glob.Match(end.Name) {
				continue
			}
			if hint.Port != "" {
				host, _, _ := net.SplitHostPort(end.Address)
				end.Address = net.JoinHostPort(host, hint.Port)
			}
			if s := hint.User; s != "" {
				end.User = s
			}
			if s := hint.ForwardAgent; s != nil {
				end.ForwardAgent = *s
			}
			if s := hint.RequestTTY; s != nil {
				end.RequestTTY = *s
			}
			if s := hint.RemoteCommand; s != "" {
				end.RemoteCommand = s
			}
			if s := hint.Desc; s != "" {
				end.Desc = s
			}
			if s := hint.Link; !reflect.DeepEqual(s, wishlist.Link{}) {
				end.Link = s
			}
			if s := hint.ProxyJump; s != "" {
				end.ProxyJump = s
			}
			end.SendEnv = append(end.SendEnv, hint.SendEnv...)
			end.SetEnv = append(end.SetEnv, hint.SetEnv...)
			end.PreferredAuthentications = append(end.PreferredAuthentications, hint.PreferredAuthentications...)
			end.IdentityFiles = append(end.IdentityFiles, hint.IdentityFiles...)
			if s := hint.Timeout; s != 0 {
				end.Timeout = s
			}
			seed[i] = end
		}
	}
	return seed
}

func getConfig(configFile string, seed []*wishlist.Endpoint) (wishlist.Config, string, error) {
	var allErrs error
	for _, path := range append([]string{configFile}, userConfigPaths()...) {
		if path == "" {
			continue
		}

		cfg, err := getConfigFile(path, seed)
		if err != nil {
			log.Info("Not using", "path", path, "err", err)
			if errors.Is(err, os.ErrNotExist) {
				allErrs = multierror.Append(allErrs, fmt.Errorf("%q: %w", path, err))
				continue
			}
			return wishlist.Config{}, "", err
		}

		log.Info("Using configuration file", "path", path)
		return cfg, path, nil
	}
	return wishlist.Config{}, "", fmt.Errorf("no valid config files found: %w", allErrs)
}

func getConfigFile(path string, seed []*wishlist.Endpoint) (wishlist.Config, error) {
	switch filepath.Ext(path) {
	case ".yaml", ".yml":
		return getYAMLConfig(path, seed)
	default:
		return getSSHConfig(path, seed)
	}
}

func getSeedEndpoints(ctx context.Context) ([]*wishlist.Endpoint, error) {
	var seed []*wishlist.Endpoint
	if tailscaleNet != "" {
		endpoints, err := tailscale.Endpoints(ctx, tailscaleNet, tailscaleKey, tailscaleClientID, tailscaleClientSecret)
		if err != nil {
			return nil, err //nolint: wrapcheck
		}
		seed = append(seed, endpoints...)
	}
	if zeroconfEnabled {
		endpoints, err := zeroconf.Endpoints(ctx, zeroconfDomain, zeroconfTimeout)
		if err != nil {
			return nil, err //nolint: wrapcheck
		}
		seed = append(seed, endpoints...)
	}
	for _, domain := range srvDomains {
		endpoints, err := srv.Endpoints(ctx, domain)
		if err != nil {
			return nil, err //nolint: wrapcheck
		}
		seed = append(seed, endpoints...)
	}
	sort.Slice(seed, func(i, j int) bool {
		return seed[i].Name < seed[j].Name
	})
	return seed, nil
}

func getYAMLConfig(path string, seed []*wishlist.Endpoint) (wishlist.Config, error) {
	var config wishlist.Config

	bts, err := os.ReadFile(path)
	if err != nil {
		return config, fmt.Errorf("failed to read config: %w", err)
	}

	if err := yaml.Unmarshal(bts, &config); err != nil {
		return config, fmt.Errorf("failed to parse config: %w", err)
	}

	config.Endpoints = append(config.Endpoints, applyHints(seed, config.Hints)...)
	return config, nil
}

func getSSHConfig(path string, seed []*wishlist.Endpoint) (wishlist.Config, error) {
	config := wishlist.Config{}
	endpoints, err := sshconfig.ParseFile(path, seed)
	if err != nil {
		return config, err //nolint: wrapcheck
	}
	config.Endpoints = endpoints
	return config, nil
}

func workLocally(config wishlist.Config, args []string) error {
	// either no args or arg is a list
	if len(args) == 0 || args[0] == "list" {
		m := wishlist.NewListing(config.Endpoints, wishlist.NewLocalSSHClient())
		_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
		return err //nolint: wrapcheck
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
