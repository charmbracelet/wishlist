package wishlist

import (
	"errors"
	"fmt"
	"net"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"
)

var (
	copyIPAddr = key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "copy address"),
	)
	enter = key.NewBinding(
		key.WithKeys("enter", "o"),
		key.WithHelp("enter/o", "connect"),
	)
)

// NewListing creates a new listing model for the given endpoints and SSH session.
// If session is nil, it is assume to be a local listing.
func NewListing(endpoints []*Endpoint, client SSHClient, r *lipgloss.Renderer) *ListModel {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Directory Listing"
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{copyIPAddr}
	}
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{enter}
	}

	m := &ListModel{
		list:      l,
		endpoints: endpoints,
		client:    client,
		styles:    makeStyles(r),
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
	width     int
	err       error
	styles    styles
}

// SetItems allows to update the listing items.
func (m *ListModel) SetItems(endpoints []*Endpoint) tea.Cmd {
	descriptors := features(endpoints)
	h := len(descriptors) + 1 // desc lines + title
	d := list.NewDefaultDelegate()
	d.SetHeight(h)
	m.list.SetDelegate(d)
	log.Debug("setting delegate height", "height", h)
	return m.list.SetItems(endpointsToListItems(endpoints, descriptors, m.styles))
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

func endpointsToListItems(endpoints []*Endpoint, descriptors []descriptor, styles styles) []list.Item {
	var items []list.Item //nolint: prealloc
	for _, endpoint := range endpoints {
		if !endpoint.Valid() {
			continue
		}
		items = append(items, ItemWrapper{
			endpoint:    endpoint,
			descriptors: descriptors,
			styles:      styles,
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
		if key.Matches(msg, copyIPAddr) && !m.list.SettingFilter() {
			if w := m.selected(); w != nil {
				host, _, _ := net.SplitHostPort(w.endpoint.Address)
				termenv.Copy(host)
				return m, m.list.NewStatusMessage(fmt.Sprintf("copied %q to the clipboard", host))
			}

			return m, nil
		}
		if key.Matches(msg, enter) {
			if m.list.SettingFilter() {
				break
			}
			w := m.selected()
			if w == nil {
				return m, nil
			}
			cmd := m.client.For(w.endpoint)
			return m, tea.Exec(cmd, func(err error) tea.Msg {
				return errMsg{err}
			})
		}

	case tea.WindowSizeMsg:
		top, right, bottom, left := m.styles.Doc.GetMargin()
		m.width = msg.Width
		m.list.SetSize(msg.Width-left-right, msg.Height-top-bottom)

	case SetEndpointsMsg:
		if cmd := m.SetItems(msg.Endpoints); cmd != nil {
			return m, cmd
		}

	case errMsg:
		if msg.err != nil {
			log.Warn("got an error", "err", msg.err)
			m.err = msg.err
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *ListModel) selected() *ItemWrapper {
	selectedItem := m.list.SelectedItem()
	if selectedItem == nil {
		return nil
	}
	w, ok := selectedItem.(ItemWrapper)
	if !ok {
		// this should never happen
		return nil
	}
	return &w
}

// View comply with tea.Model interface.
func (m *ListModel) View() string {
	if m.quitting {
		return ""
	}

	if m.err != nil {
		header := lipgloss.NewStyle().
			Width(m.width).
			Render("Something went wrong:")
		errstr := m.styles.Err.
			Width(m.width).
			Render(rootCause(m.err).Error())
		footer := m.styles.Footer.
			Width(m.width).
			Render("Press any key to go back to the list.")
		return m.styles.Logo.String() + "\n\n" +
			header + "\n\n" +
			errstr + "\n\n" +
			footer + "\n"
	}
	return m.styles.Doc.Render(m.list.View())
}

func rootCause(err error) error {
	log.Warn("root cause", "err", err)
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
