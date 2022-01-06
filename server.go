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

func multiplex(r io.Reader, done <-chan bool) (io.Reader, io.Reader) {
	var r1 bytes.Buffer
	var r2 bytes.Buffer
	w := io.MultiWriter(&r1, &r2)

	go func() {
		for {
			select {
			case <-done:
				return
			default:
				buf := [256]byte{}
				n, err := r.Read(buf[:])
				if err != nil {
					log.Println("multiplex error:", err)
					continue
				}
				if n == 0 {
					continue
				}
				if _, err := w.Write(buf[:n]); err != nil {
					log.Println("multiplex error:", err)
				}
			}
		}
	}()

	return &r1, &r2
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

			done := make(chan bool, 1)
			defer func() { done <- true }()
			listStdin, handoffStdin := multiplex(s, done)

			opts = append(opts, tea.WithInput(blockingReader{listStdin}), tea.WithOutput(s))
			p := tea.NewProgram(m, opts...)

			go func() {
				_, winch, _ := s.Pty()
				for {
					select {
					case <-s.Context().Done():
						if p != nil {
							p.Quit()
						}
						return
					case w := <-winch:
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
			log.Println("here")

			if cte := s.Context().Value(HandoffContextKey); cte != nil {
				n, err := io.ReadAll(handoffStdin) // exhaust the handoff stdin first
				log.Println("exhausted handoff stdin", len(n), err)
				// TODO: keep exhausting the other stdin?
				MustConnect(s, cte.(*Endpoint), blockingReader{handoffStdin})
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
