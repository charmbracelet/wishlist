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
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/gliderlabs/ssh"
	"github.com/hashicorp/go-multierror"
)

// handles ssh host -t appname
func cmdMiddleware(endpoints []*Endpoint) wish.Middleware {
	return func(h ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			if cmd := s.Command(); len(cmd) == 1 && cmd[0] != "list" {
				for _, e := range endpoints {
					if e.Name == cmd[0] {
						MustConnect(s, e)
					}
				}
				fmt.Fprintln(s.Stderr(), "command not found:", cmd)
				return
			}
			h(s)
		}
	}
}

// handles handoff to another app
func handoffMiddleware(h ssh.Handler) ssh.Handler {
	return func(s ssh.Session) {
		if cte := s.Context().Value(HandoffContextKey); cte != nil {
			s.Context().SetValue(QuitAppContextKey, true)
			MustConnect(s, cte.(*Endpoint))
		}
	}
}

func bubbleteaMiddleware(bth bm.BubbleTeaHandler) wish.Middleware {
	return func(h ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			errc := make(chan error, 1)
			m, opts := bth(s)
			if m != nil {
				opts = append(opts, tea.WithInput(s), tea.WithOutput(s))
				p := tea.NewProgram(m, opts...)
				_, windowChanges, _ := s.Pty()

				go func() {
					for {
						select {
						case <-s.Context().Done():
							if p != nil {
								p.Quit()
							}
							return
						case w := <-windowChanges:
							if p != nil {
								p.Send(tea.WindowSizeMsg{Width: w.Width, Height: w.Height})
							}
						case err := <-errc:
							if err != nil {
								log.Print(err)
							}
						default:
							if p == nil {
								continue
							}
							if q := s.Context().Value(QuitAppContextKey); q != nil {
								p.Quit()
								return
							}
						}
					}
				}()
				errc <- p.Start()
			}
			h(s)
		}
	}
}

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
			Middlewares: []wish.Middleware{
				handoffMiddleware,
				bubbleteaMiddleware(func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
					return newListing(config.Endpoints, s), nil
				}),
				cmdMiddleware(config.Endpoints),
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
