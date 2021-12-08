package wishlist

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/gliderlabs/ssh"
	"github.com/hashicorp/go-multierror"
)

type Endpoint struct {
	Name    string
	Address string
	Handler bm.BubbleTeaHandler
}

type Config struct {
	Listen    string
	Port      int
	Endpoints []*Endpoint
	Factory   func(Endpoint) (*ssh.Server, error)
}

func List(config *Config) error {
	var closes []func() error
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	for i, endpoint := range append([]*Endpoint{
		{
			Name:    "listing",
			Address: fmt.Sprintf("%s:%d", config.Listen, config.Port),
			Handler: func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
				return newListing(config.Endpoints), []tea.ProgramOption{tea.WithAltScreen()}
			},
		},
	}, config.Endpoints...) {
		if endpoint.Address == "" {
			endpoint.Address = fmt.Sprintf("%s:%d", config.Listen, config.Port+i)
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

var keyEnter = key.NewBinding(
	key.WithKeys("enter"),
	key.WithHelp("enter", "connect server"),
)

func newListing(endpoints []*Endpoint) tea.Model {
	var items []list.Item
	for _, endpoint := range endpoints {
		items = append(items, endpoint)
	}
	l := list.NewModel(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Directory Listing"
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{keyEnter}
	}
	return model{l, endpoints}
}

type model struct {
	list      list.Model
	endpoints []*Endpoint
}

func (i *Endpoint) Title() string       { return i.Name }
func (i *Endpoint) Description() string { return fmt.Sprintf("ssh://%s", i.Address) }
func (i *Endpoint) FilterValue() string { return i.Name }

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, keyEnter) {
			log.Println(m.list.Index())
			end := m.endpoints[m.list.Index()]
			log.Println(end.Address)
			return m, nil
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
