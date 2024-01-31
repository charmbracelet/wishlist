package wishlist

import "github.com/charmbracelet/lipgloss"

func makeStyles(r *lipgloss.Renderer) styles {
	return styles{
		Logo: r.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#FFFDF5", Dark: "#FFFDF5"}).
			Background(lipgloss.Color("#5A56E0")).
			Padding(0, 1).
			SetString("Wishlist"),
		Err: r.NewStyle().
			Italic(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"}),
		Footer: r.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"}),
		NoContent: r.NewStyle().Faint(true).Italic(true),
		Doc:       r.NewStyle().Margin(1, 2),
	}
}

type styles struct {
	Logo      lipgloss.Style
	Err       lipgloss.Style
	Footer    lipgloss.Style
	NoContent lipgloss.Style
	Doc       lipgloss.Style
}
