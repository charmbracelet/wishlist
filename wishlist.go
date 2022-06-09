package wishlist

import (
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
	d := list.NewDefaultDelegate()

	l := list.NewModel(nil, d, 0, 0)
	l.Title = "Directory Listing"
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{enter}
	}

	m := &ListModel{
		list:      l,
		delegate:  d,
		endpoints: endpoints,
		client:    client,
	}
	m.SetItems(endpoints)
	return m
}

// ListModel main wishlist model.
type ListModel struct {
	list      list.Model
	delegate  list.DefaultDelegate
	endpoints []*Endpoint
	client    SSHClient
	quitting  bool
	err       error
}

const defaultHeigth = 2

// SetItems allows to update the listing items.
func (m *ListModel) SetItems(endpoints []*Endpoint) tea.Cmd {
	hasLink, hasDesc := features(endpoints)
	h := defaultHeigth
	if hasLink {
		h++
	}
	if hasDesc {
		h++
	}
	m.delegate.SetHeight(h)
	m.list.SetDelegate(m.delegate)
	log.Println("setting delegate height:", h)
	return m.list.SetItems(endpointsToListItems(endpoints, hasLink, hasDesc))
}

func features(endpoints []*Endpoint) (hasLink bool, hasDesc bool) {
	for _, endpoint := range endpoints {
		if !endpoint.Valid() {
			continue
		}
		if endpoint.Desc != "" {
			hasDesc = true
		}
		if endpoint.Link.URL != "" {
			hasLink = true
		}
		if hasDesc && hasLink {
			break
		}
	}
	return
}

func endpointsToListItems(endpoints []*Endpoint, hasLink, hasDesc bool) []list.Item {
	var items []list.Item
	for _, endpoint := range endpoints {
		if !endpoint.Valid() {
			continue
		}
		var descriptors []descriptor
		if hasDesc {
			descriptors = append(descriptors, withDescription)
		}
		if hasLink {
			descriptors = append(descriptors, withLink)
		}
		descriptors = append(descriptors, withSSHURL)
		items = append(items, ItemWrapper{
			endpoint:    endpoint,
			descriptors: descriptors,
		})
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
			cmd := m.client.For(selectedItem.(*ItemWrapper).endpoint)
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

var boldStyle = lipgloss.NewStyle().Bold(true)

// View comply with tea.Model interface.
func (m *ListModel) View() string {
	if m.quitting {
		return ""
	}
	if m.err != nil {
		return boldStyle.Render("Something went wrong:") + "\n\n" +
			m.err.Error() + "\n\n" +
			boldStyle.Render("Press 'q' to quit.") + "\n"
	}
	return docStyle.Render(m.list.View())
}
