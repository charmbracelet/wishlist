module github.com/charmbracelet/wishlist

go 1.16

require (
	github.com/charmbracelet/bubbles v0.9.0
	github.com/charmbracelet/bubbletea v0.19.2
	github.com/charmbracelet/keygen v0.1.2
	github.com/charmbracelet/lipgloss v0.4.0
	github.com/charmbracelet/wish v0.1.1
	github.com/gliderlabs/ssh v0.3.3
	github.com/hashicorp/go-multierror v1.1.1
	github.com/muesli/termenv v0.9.0
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
)

replace github.com/charmbracelet/wish => ../wish
