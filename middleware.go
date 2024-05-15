package wishlist

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wishlist/blocking"
	"github.com/charmbracelet/wishlist/multiplex"
	"github.com/muesli/termenv"
	"github.com/teivah/broadcast"
)

// handles ssh host -t appname.
func cmdsMiddleware(endpoints []*Endpoint) wish.Middleware {
	valid := []string{`"list"`}
	for _, e := range endpoints {
		valid = append(valid, fmt.Sprintf("%q", e.Name))
	}
	return func(h ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			cmd := s.Command()

			if len(cmd) == 0 {
				h(s)
				return
			}

			if len(cmd) == 1 && cmd[0] != "list" {
				for _, e := range endpoints {
					if e.Name == cmd[0] {
						mustConnect(s, e)
						return // unreachable
					}
				}
				wish.Fatal(s, fmt.Errorf("wishlist: command %q not found, valid commands are %s", cmd[0], strings.Join(valid, ", ")))
				return // unreachable
			}
			h(s)
		}
	}
}

// handles the listing and handoff of apps.
func listingMiddleware(config *Config, endpointRelay *broadcast.Relay[[]*Endpoint]) wish.Middleware {
	return func(ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			lipgloss.SetColorProfile(termenv.ANSI256)

			multiplexDoneCh := make(chan bool, 1)
			defer func() { multiplexDoneCh <- true }()
			listStdin, handoffStdin := multiplex.Reader(s, multiplexDoneCh)

			endpointL := endpointRelay.Listener(0)
			defer endpointL.Close()

			errch := make(chan error, 1)
			appch := make(chan bool, 1)
			model := NewRemoteListing(
				config.Endpoints,
				&remoteClient{
					session: s,
					stdin:   handoffStdin,
					cleanup: func() {
						listStdin.Reset()
						handoffStdin.Reset()
					},
				},
				bm.MakeRenderer(s),
			)
			p := tea.NewProgram(
				model,
				tea.WithInput(blocking.New(listStdin)),
				tea.WithOutput(s),
				tea.WithAltScreen(),
			)
			go listenAppEvents(s, p, appch, endpointL.Ch(), errch)
			_, err := p.Run()
			errch <- err
			appch <- true
		}
	}
}

// listens and handles events:
// - donech: when the list quits
// - errch: when the program errors
// - session's context done: when the session is terminated by either party
// - winch: when the terminal is resized
// - endpointsch: new endpoint list provided
// and handles them accordingly.
func listenAppEvents(
	s ssh.Session,
	p *tea.Program,
	donech <-chan bool,
	endpointsch <-chan []*Endpoint,
	errch <-chan error,
) {
	_, winch, _ := s.Pty()
	for {
		select {
		case <-donech:
			return
		case <-s.Context().Done():
			if p != nil {
				p.Quit()
			}
			return
		case w := <-winch:
			if p != nil {
				p.Send(tea.WindowSizeMsg{Width: w.Width, Height: w.Height})
			}
		case m := <-endpointsch:
			if p != nil {
				p.Send(SetEndpointsMsg{Endpoints: m})
			}
		case err := <-errch:
			if err != nil {
				log.Print("got an err:", err)
			}
		}
	}
}

func mustConnect(session ssh.Session, e *Endpoint) {
	client := &remoteClient{
		session: session,
		stdin:   session,
	}
	cmd := client.For(e)
	cmd.SetStderr(session.Stderr())
	cmd.SetStdout(session)
	if err := cmd.Run(); err != nil {
		wish.Fatal(session, fmt.Errorf("wishlist: %w", err))
		return // unreachable
	}
	fmt.Fprintf(session, "wishlist: closed connection to %q (%s)\n\r", e.Name, e.Address)
	_ = session.Exit(0)
}
