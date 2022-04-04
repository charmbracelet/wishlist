package wishlist

import (
	"fmt"

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

// HandoffModel is a tea.Model that can tell where it should ssh into.
type HandoffModel interface {
	tea.Model
	HandoffTo() *Endpoint
}

// LocalListing creates a new listing model for local usage only.
func LocalListing(endpoints []*Endpoint) HandoffModel {
	return newListing(endpoints, nil)
}

func newListing(endpoints []*Endpoint, s ssh.Session) *ListModel {
	l := list.NewModel(endpointsToListItems(endpoints), list.NewDefaultDelegate(), 0, 0)
	l.Title = "Directory Listing"
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{enter}
	}
	return &ListModel{
		list:      l,
		endpoints: endpoints,
		session:   s,
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
	handoff   *Endpoint
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

// Update comply with tea.Model interface.
func (m *ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, enter) {
			selectedItem := m.list.SelectedItem()
			if selectedItem == nil {
				return m, nil
			}
			m.handoff = selectedItem.(*Endpoint)
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		top, right, bottom, left := docStyle.GetMargin()
		m.list.SetSize(msg.Width-left-right, msg.Height-top-bottom)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View comply with tea.Model interface.
func (m *ListModel) View() string {
	if m.handoff != nil {
		return ""
	}
	return docStyle.Render(m.list.View())
}

// HandoffTo returns which endpoint the user wants to ssh into.
func (m *ListModel) HandoffTo() *Endpoint {
	return m.handoff
}
