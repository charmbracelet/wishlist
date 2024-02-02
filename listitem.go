package wishlist

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
)

var _ list.Item = ItemWrapper{}

// ItemWrapper wrappes an Endpoint and a set of descriptors and acts as a list.Item.
type ItemWrapper struct {
	endpoint    *Endpoint
	descriptors []descriptor
	styles      styles
}

// FilterValue to abide the list.Item interface.
func (i ItemWrapper) FilterValue() string { return i.endpoint.Name }

// Title to abide the list.Item interface.
func (i ItemWrapper) Title() string { return i.endpoint.Name }

// Description to abide the list.Item interface.
func (i ItemWrapper) Description() string {
	lines := make([]string, 0, len(i.descriptors))
	for _, desc := range i.descriptors {
		lines = append(lines, desc(i.endpoint, i.styles))
	}
	return strings.Join(lines, "\n")
}

type descriptor func(e *Endpoint, styles styles) string

func withSSHURL(i *Endpoint, _ styles) string {
	return Link{URL: "ssh://" + i.Address}.String()
}

func withLink(i *Endpoint, styles styles) string {
	if l := i.Link.String(); l != "" {
		return l
	}
	return styles.NoContent.Render("no link")
}

func withDescription(i *Endpoint, styles styles) string {
	if desc := strings.Split(i.Desc, "\n")[0]; desc != "" {
		return desc
	}
	return styles.NoContent.Render("no description")
}
