package wishlist

import (
	"fmt"
	"io"
	"log"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gliderlabs/ssh"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2) // nolint:gomnd

var enter = key.NewBinding(
	key.WithKeys("enter"),
	key.WithHelp("Enter", "Connect"),
)

// NewListing creates a new listing model for the given endpoints and SSH session.
// If sessuion is nil, it is assume to be a local listing.
func NewListing(endpoints []*Endpoint, s ssh.Session, clientStdin io.Reader) *ListModel {
	l := list.NewModel(endpointsToListItems(endpoints), list.NewDefaultDelegate(), 0, 0)
	l.Title = "Directory Listing"
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{enter}
	}

	var client SSHClient
	if s == nil {
		client = &localClient{}
	} else {
		client = &remoteClient{
			session: s,
			stdin:   clientStdin,
			exhaust: true,
		}
	}

	return &ListModel{
		list:      l,
		endpoints: endpoints,
		session:   s,
		client:    client,
	}
}

// Title to abide the list.Item interface.
func (i *Endpoint) Title() string { return i.Name }

// Description to abide the list.Item interface.
func (i *Endpoint) Description() string { return fmt.Sprintf("ssh://%s", i.Address) }

// FilterValue to abide the list.Item interface.
func (i *Endpoint) FilterValue() string { return i.Name }

// ListModel main wishlist model.
type ListModel struct {
	list      list.Model
	endpoints []*Endpoint
	session   ssh.Session
	client    SSHClient
	err       error
}

// SetItems allows to update the listing items.
func (m *ListModel) SetItems(endpoints []*Endpoint) tea.Cmd {
	return m.list.SetItems(endpointsToListItems(endpoints))
}

func endpointsToListItems(endpoints []*Endpoint) []list.Item {
	var items []list.Item
	for _, endpoint := range endpoints {
		if endpoint.Valid() {
			items = append(items, endpoint)
		}
	}
	return items
}

// Init comply with tea.Model interface.
func (m *ListModel) Init() tea.Cmd {
	return nil
}

type errMsg struct {
	err error
}

func errCmd(err error) tea.Cmd {
	return func() tea.Msg {
		return errMsg{err}
	}
}

type connectMsg struct {
	endpoint *Endpoint
}

// Update comply with tea.Model interface.
func (m *ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, enter) {
			selectedItem := m.list.SelectedItem()
			if selectedItem == nil {
				return m, nil
			}
			return m, tea.Batch(tea.ReleaseTerminal(func(err error) tea.Msg {
				return errMsg{err}
			}), func() tea.Msg {
				return connectMsg{selectedItem.(*Endpoint)}
			})
		}

	case connectMsg:
		cmd, err := m.client.Connect(msg.endpoint)
		if err != nil {
			return m, errCmd(err)
		}
		return m, tea.Exec(cmd, func(err error) tea.Msg {
			return errMsg{err}
		})

	case tea.WindowSizeMsg:
		top, right, bottom, left := docStyle.GetMargin()
		m.list.SetSize(msg.Width-left-right, msg.Height-top-bottom)

	case errMsg:
		if msg.err != nil {
			log.Println("got an error:", msg.err)
			m.err = msg.err
			return m, tea.RestoreTerminal(nil)
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View comply with tea.Model interface.
func (m *ListModel) View() string {
	if m.err != nil {
		return "something went wrong:" + m.err.Error() + "\npress q to quit"
	}
	return docStyle.Render(m.list.View())
}
