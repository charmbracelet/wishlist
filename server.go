package wishlist

import (
	"bytes"
	"fmt"
	"io"
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
						MustConnect(s, e, s)
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
func handoffMiddleware(r io.Reader, p *tea.Program) func(h ssh.Handler) ssh.Handler {
	return func(h ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			if cte := s.Context().Value(HandoffContextKey); cte != nil {
				n, err := io.ReadAll(r) //exhaust the handoff stdin first
				log.Println("exhausted handoff stdin", fmt.Sprintf("%x", n), err)
				// p.Quit()
				// TODO: keep exhausting the other stdin?
				MustConnect(s, cte.(*Endpoint), blockingReader{r})
			}
		}
	}
}

func bubbleteaMiddleware(bth bm.BubbleTeaHandler) wish.Middleware {
	return func(h ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			errc := make(chan error, 1)
			m, opts := bth(s)
			if m == nil {
				h(s)
				return
			}
			var listStdin bytes.Buffer
			var handoffStdin bytes.Buffer
			stdin := io.MultiWriter(&listStdin, &handoffStdin)

			go func() {
				for {
					select {
					case <-s.Context().Done():
						return
					default:
						buf := [256]byte{}
						n, err := s.Read(buf[:])
						if err != nil {
							errc <- err
							continue
						}
						if n == 0 {
							continue
						}
						if _, err := stdin.Write(buf[:n]); err != nil {
							errc <- err
						}
					}
				}
			}()

			opts = append(opts, tea.WithInput(blockingReader{&listStdin}), tea.WithOutput(s))
			p := tea.NewProgram(m, opts...)
			_, windowChanges, _ := s.Pty()

			go func() {
				for {
					select {
					case <-s.Context().Done():
						log.Println("DONE")
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
							log.Print("got an err:", err)
						}
					}
				}
			}()

			errc <- p.Start()

			if cte := s.Context().Value(HandoffContextKey); cte != nil {
				n, err := io.ReadAll(&handoffStdin) //exhaust the handoff stdin first
				log.Println("exhausted handoff stdin", len(n), err)
				// p.Quit()
				// TODO: keep exhausting the other stdin?
				log.Println("blocks")
				MustConnect(s, cte.(*Endpoint), blockingReader{&handoffStdin})
				log.Println("unblocks")
			}
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
				// handoffMiddleware,
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
