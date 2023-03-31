package wishlist

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/promwish"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/hashicorp/go-multierror"
	"github.com/teivah/broadcast"
)

// Serve serves wishlist with the given config.
func Serve(config *Config) error {
	var closes []func() error
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	if config.Port == 0 {
		port, err := getFirstOpenPort(config.Listen, 22, 2222) //nolint:gomnd
		if err != nil {
			return fmt.Errorf("could not get an open port and none was provided: %w", err)
		}
		config.Port = port
	}

	if config.Listen == "" {
		config.Listen = "0.0.0.0"
	}

	if err := os.MkdirAll(".wishlist", 0o700); err != nil { //nolint:gomnd
		return fmt.Errorf("could not create .wishlist dir: %w", err)
	}

	relay := broadcast.NewRelay[[]*Endpoint]()
	if config.EndpointChan != nil {
		go func() {
			for endpoints := range config.EndpointChan {
				config.Endpoints = endpoints
				relay.Broadcast(endpoints)
			}
		}()
	}

	config.lastPort = config.Port
	for _, endpoint := range append([]*Endpoint{
		{
			Name:    "list",
			Address: toAddress(config.Listen, config.Port),
			Middlewares: []wish.Middleware{
				listingMiddleware(config, relay),
				cmdsMiddleware(config.Endpoints),
			},
		},
	}, config.Endpoints...) {
		if endpoint.Name == "list" && config.Metrics.Enabled {
			endpoint.Middlewares = append(endpoint.Middlewares, promwish.Middleware(
				FirstNonEmpty(config.Metrics.Address, "localhost:9222"),
				FirstNonEmpty(config.Metrics.Name, "wishlist"),
			))
		}

		if !endpoint.Valid() || !endpoint.ShouldListen() {
			continue
		}

		if endpoint.Address == "" {
			endpoint.Address = toAddress(config.Listen, atomic.AddInt64(&config.lastPort, 1))
		}

		// i don't see where closer was declared before, linter bug maybe?
		closer, err := listenAndServe(config, *endpoint) //nolint:predeclared
		if closer != nil {
			closes = append(closes, closer)
		}
		if err != nil {
			if err2 := closeAll(closes); err2 != nil && !errors.Is(err2, ssh.ErrServerClosed) {
				return multierror.Append(err, err2)
			}
			return err
		}
	}
	<-done
	log.Info("Stopping SSH servers")
	return closeAll(closes)
}

// listenAndServe starts a server for the given endpoint.
func listenAndServe(config *Config, endpoint Endpoint) (func() error, error) {
	s, err := config.Factory(endpoint)
	if err != nil {
		return nil, err
	}
	s.PublicKeyHandler = publicKeyAccessOption(config.Users)

	log.Info("Starting SSH server", "endpoint", endpoint.Name, "address", "ssh://"+endpoint.Address)
	ln, err := net.Listen("tcp", endpoint.Address)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}
	go func() {
		if err := s.Serve(ln); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("SSH server failed", "err", err)
		}
	}()

	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) //nolint:gomnd
		defer func() { cancel() }()
		return s.Shutdown(ctx) //nolint:wrapcheck
	}, nil
}

// runs all the close functions and returns all errors.
func closeAll(closes []func() error) error {
	var result error
	for _, close := range closes {
		if err := close(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			result = multierror.Append(result, err)
		}
	}
	return result //nolint:wrapcheck
}

// returns `listen:port`.
func toAddress(listen string, port int64) string {
	return net.JoinHostPort(listen, fmt.Sprintf("%d", port))
}

func getFirstOpenPort(addr string, ports ...int64) (int64, error) {
	for _, port := range ports {
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", addr, port))
		if err != nil {
			continue
		}

		// port seems available
		if err := ln.Close(); err != nil {
			return 0, err //nolint:wrapcheck
		}

		return port, nil
	}

	return 0, fmt.Errorf("all ports unavailable")
}

func publicKeyAccessOption(users []User) ssh.PublicKeyHandler {
	if len(users) == 0 {
		// if no users, assume everyone can login
		return nil
	}

	return func(ctx ssh.Context, key ssh.PublicKey) bool {
		for _, user := range users {
			if user.Name == ctx.User() {
				for _, pubkey := range user.PublicKeys {
					upk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubkey))
					if err != nil {
						log.Warn("invalid key", "user", user.Name, "err", err)
						return false
					}
					if ssh.KeysEqual(upk, key) {
						log.Info("authorized", "user", ctx.User(), "key", pubkey[:30])
						return true
					}
				}
			}
		}
		log.Warn("denied", "user", ctx.User(), "key.type", key.Type())
		return false
	}
}
