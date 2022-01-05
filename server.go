package wishlist

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gliderlabs/ssh"
	"github.com/hashicorp/go-multierror"
)

// Serve servers the list for the given config.
func Serve(config *Config) error {
	var closes []func() error
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	config.lastPort = config.Port
	for _, endpoint := range append([]*Endpoint{
		{
			Name:    "list",
			Address: toAddress(config.Listen, config.Port),
			Handler: func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
				cmd := s.Command()
				if len(cmd) == 1 && cmd[0] != "list" {
					for _, e := range config.Endpoints {
						if e.Name == cmd[0] {
							if e.Handler != nil {
								return e.Handler(s)
							}
							if e.Address != "" {
								mustConnect(s, e)
								return nil, nil
							}
						}
					}
				}
				log.Println("command not found:", cmd)
				return newListing(config.Endpoints, s), nil
			},
		},
	}, config.Endpoints...) {
		if !endpoint.Valid() || !endpoint.ShouldListen() {
			continue
		}

		if endpoint.Address == "" {
			endpoint.Address = toAddress(config.Listen, atomic.AddInt64(&config.lastPort, 1))
		}

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

func closeAll(closes []func() error) error {
	var result error
	for _, close := range closes {
		if err := close(); err != nil {
			result = multierror.Append(result, err)
		}
	}
	return result
}

func toAddress(listen string, port int64) string {
	return fmt.Sprintf("%s:%d", listen, port)
}
