package wishlist

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

var noContentStyle = lipgloss.NewStyle().Faint(true).Italic(true)

var _ list.Item = ItemWrapper{}

type ItemWrapper struct {
	endpoint    *Endpoint
	descriptors []descriptor
}

// FilterValue to abide the list.Item interface.
func (i ItemWrapper) FilterValue() string { return i.endpoint.Name }

// Title to abide the list.Item interface.
func (i ItemWrapper) Title() string { return i.endpoint.Name }

// Description to abide the list.Item interface.
func (i ItemWrapper) Description() string {
	var lines []string
	for _, desc := range i.descriptors {
		lines = append(lines, desc(i.endpoint))
	}
	return strings.Join(lines, "\n")
}

type descriptor func(e *Endpoint) string

func withSSHURL(i *Endpoint) string {
	return Link{URL: "ssh://" + i.Address}.String()
}

func withLink(i *Endpoint) string {
	if l := i.Link.String(); l != "" {
		return l
	}
	return noContentStyle.Render("no link")
}

func withDescription(i *Endpoint) string {
	if desc := strings.Split(i.Desc, "\n")[0]; desc != "" {
		return desc
	}
	return noContentStyle.Render("no description")
}