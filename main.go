package wishlist

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/gliderlabs/ssh"
	"github.com/hashicorp/go-multierror"
)

// Endpoint represents an endpoint to list.
// If it has a Handler, wishlist will start an SSH server on the given address.
type Endpoint struct {
	Name    string              `json:"name"`
	Address string              `json:"address"`
	Handler bm.BubbleTeaHandler `json:"-"`
}

// Returns true if the endpoint is valid.
func (e Endpoint) Valid() bool {
	return e.Name != "" && (e.Handler != nil || e.Address != "")
}

// ShouldListen returns true if we should start a server for this endpoint.
func (e Endpoint) ShouldListen() bool {
	return e.Handler != nil
}

// Config represents the wishlist configuration.
type Config struct {
	Listen    string                              `json:"listen"`
	Port      int64                               `json:"port"`
	Endpoints []*Endpoint                         `json:"endpoints"`
	Factory   func(Endpoint) (*ssh.Server, error) `json:"-"`

	lastPort int64
}

// Serve servers the list for the given config.
func Serve(config *Config) error {
	var closes []func() error
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	config.lastPort = config.Port
	for _, endpoint := range append([]*Endpoint{
		{
			Name:    "listing",
			Address: toAddress(config.Listen, config.Port),
			Handler: func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
				return newListing(config.Endpoints, s), []tea.ProgramOption{tea.WithAltScreen()}
			},
		},
	}, config.Endpoints...) {
		if !endpoint.Valid() || !endpoint.ShouldListen() {
			continue
		}

		if endpoint.Address == "" {
			endpoint.Address = toAddress(config.Listen, atomic.AddInt64(&config.lastPort, 1))
		}

		close, err := listen(config, *endpoint)
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

func listen(config *Config, endpoint Endpoint) (func() error, error) {
	s, err := config.Factory(endpoint)
	if err != nil {
		return nil, err
	}
	log.Printf("Starting SSH server for %s on ssh://%s", endpoint.Name, endpoint.Address)
	go s.ListenAndServe()
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

var docStyle = lipgloss.NewStyle().Margin(1, 2)

var enter = key.NewBinding(
	key.WithKeys("enter"),
	key.WithHelp("Enter", "SSH"),
)

func newListing(endpoints []*Endpoint, s ssh.Session) tea.Model {
	var items []list.Item
	for _, endpoint := range endpoints {
		if endpoint.Valid() {
			items = append(items, endpoint)
		}
	}
	l := list.NewModel(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Directory Listing"
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{enter}
	}
	return model{
		list:      l,
		endpoints: endpoints,
		session:   s,
	}
}

type model struct {
	list      list.Model
	endpoints []*Endpoint
	session   ssh.Session
}

func (i *Endpoint) Title() string       { return i.Name }
func (i *Endpoint) Description() string { return fmt.Sprintf("ssh://%s", i.Address) }
func (i *Endpoint) FilterValue() string { return i.Name }

func (m model) Init() tea.Cmd {
	return nil
}

func connectCmd(sess ssh.Session, name, addr string) tea.Cmd {
	return func() tea.Msg {
		log.Println("connecting to", addr)
		if err := connect(sess, addr); err != nil {
			fmt.Fprintln(sess, err.Error())
			sess.Exit(1)
			return nil // unreachable
		}
		log.Printf("finished connection to %q (%s)", name, addr)
		fmt.Fprintf(sess, "Closed connection to %q (%s)\n", name, addr)
		sess.Exit(0)
		return nil // unreachable
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, enter) {
			e := m.list.SelectedItem().(*Endpoint)
			return noopModel{}, connectCmd(m.session, e.Name, e.Address)
		}
	case tea.WindowSizeMsg:
		top, right, bottom, left := docStyle.GetMargin()
		m.list.SetSize(msg.Width-left-right, msg.Height-top-bottom)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return docStyle.Render(m.list.View())
}

func toAddress(listen string, port int64) string {
	return fmt.Sprintf("%s:%d", listen, port)
}

type noopModel struct{}

func (noopModel) Init() tea.Cmd                         { return nil }
func (m noopModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (noopModel) View() string                          { return "" }
