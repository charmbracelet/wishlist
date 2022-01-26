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

var docStyle = lipgloss.NewStyle().Margin(1, 2) // nolint:gomnd

var enter = key.NewBinding(
	key.WithKeys("enter"),
	key.WithHelp("Enter", "Connect"),
)

// LocalListing creates a new listing model for local usage only.
func LocalListing(endpoints []*Endpoint) tea.Model {
	return newListing(endpoints, nil)
}

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
	if m.handoff != nil {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, enter) {
			selectedItem := m.list.SelectedItem()
			if selectedItem == nil {
				return m, nil
			}
			m.handoff = selectedItem.(*Endpoint)
			if m.session == nil {
				// local run
				return m, tea.Sequentially(func() tea.Msg {
					client := &localClient{}
					if err := client.Connect(m.handoff); err != nil {
						log.Println(err)
					}
					return nil
				}, tea.Quit)
			}
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
	if m.handoff != nil {
		return ""
	}
	return docStyle.Render(m.list.View())
}
