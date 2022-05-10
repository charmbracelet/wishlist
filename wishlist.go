package wishlist

import (
	"fmt"
	"log"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2) // nolint:gomnd

var enter = key.NewBinding(
	key.WithKeys("enter"),
	key.WithHelp("Enter", "Connect"),
)

// NewListing creates a new listing model for the given endpoints and SSH session.
// If sessuion is nil, it is assume to be a local listing.
func NewListing(endpoints []*Endpoint, client SSHClient) *ListModel {
	l := list.NewModel(endpointsToListItems(endpoints), list.NewDefaultDelegate(), 0, 0)
	l.Title = "Directory Listing"
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{enter}
	}

	return &ListModel{
		list:      l,
		endpoints: endpoints,
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
	client    SSHClient
	quitting  bool
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

// SetEndpointsMsg can be used to update the listed wishlist endpoints.
type SetEndpointsMsg struct {
	Endpoints []*Endpoint
}

// Update comply with tea.Model interface.
func (m *ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, list.DefaultKeyMap().Quit) {
			m.quitting = true
		}
		if key.Matches(msg, enter) {
			selectedItem := m.list.SelectedItem()
			if selectedItem == nil {
				return m, nil
			}
			cmd := m.client.For(selectedItem.(*Endpoint))
			return m, tea.Exec(cmd, func(err error) tea.Msg {
				return errMsg{err}
			})
		}

	case tea.WindowSizeMsg:
		top, right, bottom, left := docStyle.GetMargin()
		m.list.SetSize(msg.Width-left-right, msg.Height-top-bottom)

	case SetEndpointsMsg:
		if cmd := m.SetItems(msg.Endpoints); cmd != nil {
			return m, cmd
		}

	case errMsg:
		if msg.err != nil {
			log.Println("got an error:", msg.err)
			m.err = msg.err
			return m, m.list.SetItems(nil)
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View comply with tea.Model interface.
func (m *ListModel) View() string {
	if m.quitting {
		return ""
	}
	if m.err != nil {
		return "something went wrong:" + m.err.Error() + "\npress q to quit"
	}
	return docStyle.Render(m.list.View())
}
