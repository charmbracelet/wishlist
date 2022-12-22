package wishlist

import (
	"errors"
	"log"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2) // nolint:gomnd

var (
	enter = key.NewBinding(
		key.WithKeys("enter", "o"),
		key.WithHelp("enter/o", "connect"),
	)
	keyO = key.NewBinding(
		key.WithKeys("o"),
	)
)

// NewListing creates a new listing model for the given endpoints and SSH session.
// If sessuion is nil, it is assume to be a local listing.
func NewListing(endpoints []*Endpoint, client SSHClient) *ListModel {
	l := list.NewModel(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Directory Listing"
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{enter}
	}

	m := &ListModel{
		list:      l,
		endpoints: endpoints,
		client:    client,
	}
	m.SetItems(endpoints)
	return m
}

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
	descriptors := features(endpoints)
	h := len(descriptors) + 1 // desc lines + title
	d := list.NewDefaultDelegate()
	d.SetHeight(h)
	m.list.SetDelegate(d)
	log.Println("setting delegate height:", h)
	return m.list.SetItems(endpointsToListItems(endpoints, descriptors))
}

func features(endpoints []*Endpoint) []descriptor {
	var hasDesc bool
	var hasLink bool
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

	var descriptors []descriptor
	if hasDesc {
		descriptors = append(descriptors, withDescription)
	}
	if hasLink {
		descriptors = append(descriptors, withLink)
	}
	return append(descriptors, withSSHURL)
}

func endpointsToListItems(endpoints []*Endpoint, descriptors []descriptor) []list.Item {
	var items []list.Item // nolint: prealloc
	for _, endpoint := range endpoints {
		if !endpoint.Valid() {
			continue
		}
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
		if m.err != nil {
			m.err = nil
			return m, nil
		}
		if key.Matches(msg, list.DefaultKeyMap().Quit) && !m.list.SettingFilter() && m.list.FilterState() != list.FilterApplied {
			m.quitting = true
		}
		if key.Matches(msg, enter) {
			if key.Matches(msg, keyO) && m.list.SettingFilter() {
				break
			}
			selectedItem := m.list.SelectedItem()
			if selectedItem == nil {
				return m, nil
			}
			w, ok := selectedItem.(ItemWrapper)
			if !ok {
				// this should never happen
				return m, nil
			}
			cmd := m.client.For(w.endpoint)
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
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

var (
	logoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#FFFDF5", Dark: "#FFFDF5"}).
			Background(lipgloss.Color("#5A56E0")).
			Padding(0, 1).
			SetString("Wishlist")
	errStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"})
	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"})
)

// View comply with tea.Model interface.
func (m *ListModel) View() string {
	if m.quitting {
		return ""
	}

	if m.err != nil {
		return logoStyle.String() + "\n\n" +
			"Something went wrong:" + "\n\n" +
			errStyle.Render(rootCause(m.err).Error()) + "\n\n" +
			footerStyle.Render("Press any key to go back to the list.") + "\n"
	}
	return docStyle.Render(m.list.View())
}

func rootCause(err error) error {
	log.Println("error:", err)
	for {
		e := errors.Unwrap(err)
		if e == nil {
			return err
		}
		err = e
	}
}

// FirstNonEmpty returns the first non-empty string of the list.
func FirstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
