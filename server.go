package wishlist

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/charmbracelet/wish"
	"github.com/hashicorp/go-multierror"
)

// Serve serves wishlist with the given config.
func Serve(config *Config) error {
	var closes []func() error
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	if config.Port == 0 {
		config.Port = 22 // default SSH port
	}

	config.lastPort = config.Port
	for _, endpoint := range append([]*Endpoint{
		{
			Name:    "list",
			Address: toAddress(config.Listen, config.Port),
			Middlewares: []wish.Middleware{
				listingMiddleware(config.Endpoints),
				cmdsMiddleware(config.Endpoints),
			},
		},
	}, config.Endpoints...) {
		if !endpoint.Valid() || !endpoint.ShouldListen() {
			continue
		}

		if endpoint.Address == "" {
			endpoint.Address = toAddress(config.Listen, atomic.AddInt64(&config.lastPort, 1))
		}

		// i don't see where close was declared before, linter bug maybe?
		// nolint:predeclared
		close, err := listenAndServe(config, *endpoint)
		if close != nil {
			closes = append(closes, close)
		}
		if err != nil {
			if err2 := closeAll(closes); err2 != nil {
				return multierror.Append(err, err2)
			}
			return err
		}
	}
	<-done
	log.Print("Stopping SSH servers")
	return closeAll(closes)
}

// listenAndServe starts a server for the given endpoint.
func listenAndServe(config *Config, endpoint Endpoint) (func() error, error) {
	s, err := config.Factory(endpoint)
	if err != nil {
		return nil, err
	}
	log.Printf("Starting SSH server for %s on ssh://%s", endpoint.Name, endpoint.Address)
	ln, err := net.Listen("tcp", endpoint.Address)
	if err != nil {
		return nil, err
	}
	go func() {
		if err := s.Serve(ln); err != nil {
			log.Println("SSH server error:", err)
		}
	}()
	return s.Close, nil
}

// runs all the close functions and returns all errors.
func closeAll(closes []func() error) error {
	var result error
	for _, close := range closes {
		if err := close(); err != nil {
			result = multierror.Append(result, err)
		}
	}
	return result
}

// returns `listen:port`.
func toAddress(listen string, port int64) string {
	return fmt.Sprintf("%s:%d", listen, port)
}
