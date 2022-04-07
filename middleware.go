package wishlist

import (
	"fmt"
	"log"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wishlist/blocking"
	"github.com/charmbracelet/wishlist/multiplex"
	"github.com/gliderlabs/ssh"
	"github.com/muesli/termenv"
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
				fmt.Fprintf(s.Stderr(), "wishlist: command %q not found, valid commands are %s.\n\r", cmd[0], strings.Join(valid, ", "))
				_ = s.Exit(1)
				return // unreachable
			}
			h(s)
		}
	}
}

// handles the listing and handoff of apps.
func listingMiddleware(endpoints []*Endpoint) wish.Middleware {
	return func(h ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			lipgloss.SetColorProfile(termenv.ANSI256)

			plexch := make(chan bool, 1)
			defer func() { plexch <- true }()
			listStdin, handoffStdin := multiplex.Reader(s, plexch)

			errch := make(chan error, 1)
			appch := make(chan bool, 1)
			model := NewListing(endpoints, s, handoffStdin)
			p := tea.NewProgram(
				model,
				tea.WithInput(blocking.New(listStdin)),
				tea.WithOutput(s),
				tea.WithAltScreen(),
			)
			go listenAppEvents(s, p, appch, errch)
			errch <- p.Start()
			appch <- true
		}
	}
}

// listens and handles events:
// - donech: when the list quits
// - errch: when the program errors
// - session's context done: when the session is terminated by either party
// - winch: when the terminal is resized
// and handles them accordingly.
func listenAppEvents(s ssh.Session, p *tea.Program, donech <-chan bool, errch <-chan error) {
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
	cmd, err := client.Connect(e)
	if err != nil {
		fmt.Fprintf(session, "wishlist: %s\n\r", err.Error())
		_ = session.Exit(1)
		return // unreachable
	}
	cmd.SetStderr(session.Stderr())
	cmd.SetStdout(session)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(session, "wishlist: %s\n\r", err.Error())
		_ = session.Exit(1)
		return // unreachable
	}
	fmt.Fprintf(session, "wishlist: closed connection to %q (%s)\n\r", e.Name, e.Address)
	_ = session.Exit(0)
}
