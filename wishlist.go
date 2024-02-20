package wishlist

import (
	"errors"
	"fmt"
	"net"
	"os/user"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var (
	enter = key.NewBinding(
		key.WithKeys("enter", "o"),
		key.WithHelp("enter/o", "connect"),
	)
	keyN = key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "new temporary host"),
	)
	keyE = key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit and connect"),
	)
)

//	NewLocalListing creates a new local listing model for the given
//
// endpoints and SSH session.
// A local list allow to edit before connecting, as well as creating new
// endpoints on the go.
// If session is nil, it is assume to be a local listing.
func NewLocalListing(endpoints []*Endpoint, client SSHClient, r *lipgloss.Renderer) *ListModel {
	m := newListing(endpoints, client, r)
	m.local = true
	m.list.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{enter, keyE, keyN}
	}
	return m
}

// NewRemoteListing creates a new remote listing model for the given
// endpoints and SSH session.
// If session is nil, it is assume to be a local listing.
func NewRemoteListing(endpoints []*Endpoint, client SSHClient, r *lipgloss.Renderer) *ListModel {
	return newListing(endpoints, client, r)
}

// NewListing creates a new listing model for the given endpoints and SSH session.
// If session is nil, it is assume to be a local listing.
func newListing(endpoints []*Endpoint, client SSHClient, r *lipgloss.Renderer) *ListModel {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Directory Listing"
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

	local   bool
	form    *huh.Form
	editing *editableEndpoint
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

type connectMsg struct {
	endpoint *Endpoint
}

// Update comply with tea.Model interface.
func (m *ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.form != nil {
		form, cmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}

		if m.form.State == huh.StateNormal {
			return m, cmd
		}

		if m.form.State == huh.StateAborted {
			m.form = nil
			m.editing = nil
			return m, nil
		}

		m.form = nil
		return m, m.connectToEdited
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.err != nil {
			m.err = nil
			return m, nil
		}
		if key.Matches(msg, list.DefaultKeyMap().Quit) && !m.list.SettingFilter() && m.list.FilterState() != list.FilterApplied {
			m.quitting = true
		}
		if m.local && key.Matches(msg, keyN) {
			return m.customize(&Endpoint{})
		}
		if m.local && key.Matches(msg, keyE) {
			selectedItem := m.list.SelectedItem()
			if selectedItem == nil {
				return m, nil
			}
			w := selectedItem.(ItemWrapper)
			return m.customize(w.endpoint)
		}
		if key.Matches(msg, enter) {
			if m.list.SettingFilter() {
				break
			}
			selectedItem := m.list.SelectedItem()
			if selectedItem == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return connectMsg{selectedItem.(ItemWrapper).endpoint}
			}
		}

	case connectMsg:
		m.editing = nil
		cmd := m.client.For(msg.endpoint)
		return m, tea.Exec(cmd, func(err error) tea.Msg {
			return errMsg{err}
		})

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

// View comply with tea.Model interface.
func (m *ListModel) View() string {
	if m.quitting {
		return ""
	}

	if m.form != nil {
		return m.form.View()
	}

	if m.err != nil {
		header := lipgloss.NewStyle().
			Width(m.width).
			Render("Something went wrong:")
		errstr := m.styles.Err.Copy().
			Width(m.width).
			Render(rootCause(m.err).Error())
		footer := m.styles.Footer.Copy().
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

type editableEndpoint struct {
	host                     string
	port                     string
	user                     string
	remoteCommand            string
	proxyJump                string
	sendEnv                  string
	setEnv                   string
	preferredAuthentications string
	identityFiles            string
	timeout                  string
	forwardAgent             bool
	requestTTY               bool
}

func (m *ListModel) customize(endpoint *Endpoint) (tea.Model, tea.Cmd) {
	host, port, _ := net.SplitHostPort(endpoint.Address)
	if port == "" {
		port = "22"
	}
	if host == "" {
		host = "localhost"
	}
	timeout := endpoint.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	user, _ := user.Current()
	advanced := false
	hideFunc := func() bool { return !advanced }

	ed := editableEndpoint{
		host,
		port,
		FirstNonEmpty(endpoint.User, user.Username),
		endpoint.RemoteCommand,
		endpoint.ProxyJump,
		strings.Join(endpoint.SendEnv, "\n"),
		strings.Join(endpoint.SetEnv, "\n"),
		strings.Join(endpoint.Authentications(), "\n"),
		strings.Join(endpoint.IdentityFiles, "\n"),
		timeout.String(),
		endpoint.ForwardAgent,
		endpoint.RequestTTY,
	}
	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Hostname").
				Value(&ed.host),
			huh.NewInput().
				Title("User").
				Value(&ed.user),
			huh.NewConfirm().
				Title("Show advanced options").
				Value(&advanced),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Port").
				Value(&ed.port),
			huh.NewInput().
				Title("RemoteCommand").
				Value(&ed.remoteCommand),
			huh.NewInput().
				Title("Timeout").
				Value(&ed.timeout),
		).WithHideFunc(hideFunc),
		huh.NewGroup(
			huh.NewConfirm().
				Title("ForwardAgent").
				Value(&ed.forwardAgent),
			huh.NewConfirm().
				Title("RequestTTY").
				Value(&ed.requestTTY),
			huh.NewInput().
				Title("ProxyJump").
				Value(&ed.proxyJump),
		).WithHideFunc(hideFunc),
		huh.NewGroup(
			huh.NewText().
				Title("SendEnv").
				Description("One item per line").
				Value(&ed.sendEnv),
			huh.NewText().
				Title("SetEnv").
				Description("One item per line").
				Value(&ed.setEnv),
		).WithHideFunc(hideFunc),
		huh.NewGroup(
			huh.NewText().
				Title("PreferredAuthentications").
				Description("One item per line").
				Value(&ed.preferredAuthentications),
			huh.NewText().
				Title("IdentityFiles").
				Description("One item per line, defaults to ~/.ssh/id_*").
				Value(&ed.identityFiles),
		).WithHideFunc(hideFunc),
	)
	m.editing = &ed
	return m, m.form.Init()
}

func (m *ListModel) connectToEdited() tea.Msg {
	e := m.editing
	if e == nil {
		return errMsg{fmt.Errorf("no endpoint information")}
	}
	d, err := time.ParseDuration(e.timeout)
	if err != nil {
		return errMsg{err}
	}
	split := func(s string) []string {
		var ss []string
		for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			ss = append(ss, l)
		}

		return ss
	}

	return connectMsg{&Endpoint{
		Address:                  net.JoinHostPort(e.host, e.port),
		User:                     e.user,
		ForwardAgent:             e.forwardAgent,
		RequestTTY:               e.requestTTY,
		RemoteCommand:            e.remoteCommand,
		ProxyJump:                e.proxyJump,
		SendEnv:                  split(e.sendEnv),
		SetEnv:                   split(e.setEnv),
		PreferredAuthentications: split(e.preferredAuthentications),
		IdentityFiles:            split(e.identityFiles),
		Timeout:                  d,
	}}
}
