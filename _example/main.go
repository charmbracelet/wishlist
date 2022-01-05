package main

import (
	"fmt"
	"log"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/accesscontrol"
	"github.com/charmbracelet/wish/activeterm"
	bm "github.com/charmbracelet/wish/bubbletea"
	lm "github.com/charmbracelet/wish/logging"
	"github.com/charmbracelet/wishlist"
	"github.com/gliderlabs/ssh"
)

func main() {
	if err := wishlist.Serve(&wishlist.Config{
		Listen: "127.0.0.1",
		Port:   2222,
		Factory: func(e wishlist.Endpoint) (*ssh.Server, error) {
			return wish.NewServer(
				wish.WithAddress(e.Address),
				wish.WithMiddleware(
					bm.Middleware(e.Handler),
					lm.Middleware(),
					accesscontrol.Middleware(),
					activeterm.Middleware(),
				),
			)
		},
		Endpoints: []*wishlist.Endpoint{
			{
				Name: "example app",
				Handler: func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
					return initialModel(), []tea.ProgramOption{}
				},
			},
			{
				Name:    "foo bar",
				Address: "some.other.server:2222",
			},
			{
				Name: "entries without handlers and without addresses are ignored",
			},
			{
				Address: "entries without names are ignored",
			},
		},
	}); err != nil {
		log.Fatalln(err)
	}
}

type model struct {
	spinner spinner.Model
}

func initialModel() model {
	s := spinner.NewModel()
	s.Spinner = spinner.Dot
	return model{spinner: s}
}

func (m model) Init() tea.Cmd {
	return spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Println("keypress:", msg)
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m model) View() string {
	str := fmt.Sprintf("\n\n   %s Loading forever...press q to quit\n\n", m.spinner.View())
	return str
}
