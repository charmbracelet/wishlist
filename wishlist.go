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

func newListing(endpoints []*Endpoint, s ssh.Session) *listModel {
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
	return &listModel{
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

type listModel struct {
	list      list.Model
	endpoints []*Endpoint
	session   ssh.Session
	handoff   *Endpoint
}

func (m *listModel) Init() tea.Cmd {
	return nil
}

func (m *listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m *listModel) View() string {
	return docStyle.Render(m.list.View())
}
