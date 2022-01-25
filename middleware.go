package wishlist

import (
	"fmt"
	"io"
	"log"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/wish"
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
						mustConnect(s, e, s)
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
			listStdin, handoffStdin := multiplex(s, plexch)

			errch := make(chan error, 1)
			appch := make(chan bool, 1)
			model := NewListing(endpoints, s)
			p := tea.NewProgram(
				model,
				tea.WithInput(newBlockingReader(listStdin)),
				tea.WithOutput(s),
				tea.WithAltScreen(),
			)
			go listenAppEvents(s, p, appch, errch)
			errch <- p.Start()
			appch <- true

			if endpoint := model.handoff; endpoint != nil {
				_, _ = io.ReadAll(handoffStdin) // exhaust the handoff stdin first
				mustConnect(s, endpoint, newBlockingReader(handoffStdin))
			}
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
