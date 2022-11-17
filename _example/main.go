package main

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/keygen"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	bm "github.com/charmbracelet/wish/bubbletea"
	lm "github.com/charmbracelet/wish/logging"
	"github.com/charmbracelet/wishlist"
)

func main() {
	k, err := keygen.New(filepath.Join(".wishlist", "server"), nil, keygen.Ed25519)
	if err != nil {
		log.Fatalln(err)
	}
	if !k.KeyPairExists() {
		if err := k.WriteKeys(); err != nil {
			log.Fatalln(err)
		}
	}

	// wishlist config
	cfg := &wishlist.Config{
		Factory: func(e wishlist.Endpoint) (*wish.Server, error) {
			return wish.NewServer(
				wish.WithAddress(e.Address),
				wish.WithHostKeyPEM(k.PrivateKeyPEM()),
				wish.WithPublicKeyAuth(func(ctx wish.Context, key wish.PublicKey) bool {
					return true
				}),
				wish.WithMiddleware(
					append(
						e.Middlewares, // this is the important bit: the middlewares from the endpoint
						lm.Middleware(),
						activeterm.Middleware(),
					)...,
				),
			)
		},
		Endpoints: []*wishlist.Endpoint{
			{
				Name: "bubbletea",
				Middlewares: []wish.Middleware{
					bm.Middleware(func(wish.Session) (tea.Model, []tea.ProgramOption) {
						return initialModel(), nil
					}),
				},
			},
			{
				Name: "simple",
				Middlewares: []wish.Middleware{
					func(h ssh.Handler) ssh.Handler {
						return func(s wish.Session) {
							_, _ = s.Write([]byte("hello, world\n\r"))
							h(s)
						}
					},
				},
			},
			{
				Name:    "app2",
				Address: "app.addr:2222",
			},
			{
				Name:    "server1",
				Address: "server1:22",
			},
			{
				Name:    "server2",
				Address: "server1:22",
				User:    "override_user",
			},
			{
				Name: "entries without middlewares and addresses are ignored",
			},
			{
				Address: "entries without names are ignored",
			},
		},
	}

	// start all the servers
	if err := wishlist.Serve(cfg); err != nil {
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
	case tea.WindowSizeMsg:
		log.Println("window size:", msg)
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m model) View() string {
	str := fmt.Sprintf("\n\n   %s Loading forever...press q to quit\n\n", m.spinner.View())
	return str
}
