package wishlist

import (
	"fmt"
	"log"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gliderlabs/ssh"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

var enter = key.NewBinding(
	key.WithKeys("enter"),
	key.WithHelp("Enter", "Connect"),
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
	return listModel{
		list:      l,
		endpoints: endpoints,
		session:   s,
	}
}

func (i *Endpoint) Title() string       { return i.Name }
func (i *Endpoint) Description() string { return fmt.Sprintf("ssh://%s", i.Address) }
func (i *Endpoint) FilterValue() string { return i.Name }

type listModel struct {
	list      list.Model
	endpoints []*Endpoint
	session   ssh.Session
}

func (m listModel) Init() tea.Cmd {
	return nil
}

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, enter) {
			e := m.list.SelectedItem().(*Endpoint)
			return connectedModel{}, connectCmd(m.session, e)
		}
	case tea.WindowSizeMsg:
		top, right, bottom, left := docStyle.GetMargin()
		m.list.SetSize(msg.Width-left-right, msg.Height-top-bottom)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m listModel) View() string {
	return docStyle.Render(m.list.View())
}

type connectedModel struct{}

func (connectedModel) Init() tea.Cmd { return nil }
func (connectedModel) View() string  { return "" }
func (m connectedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Println("ign msg:", msg)
	return m, nil
}

//
// cmds
//

func connectCmd(sess ssh.Session, e *Endpoint) tea.Cmd {
	return func() tea.Msg {
		log.Printf("connecting to %q (%s)", e.Name, e.Address)
		if err := connect(sess, e); err != nil {
			fmt.Fprintln(sess, err.Error())
			sess.Exit(1)
			return nil // unreachable
		}
		log.Printf("finished connection to %q (%s)", e.Name, e.Address)
		fmt.Fprintf(sess, "Closed connection to %q (%s)\n", e.Name, e.Address)
		sess.Exit(0)
		return nil // unreachable
	}
}
