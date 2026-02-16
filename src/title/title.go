package title

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Version is the current version of the application
// Update this when releasing new versions
const Version = "0.1.0"

// Animator handles the animated title state and rendering
type Animator struct {
	animFrame  int
	animOffset int
}

// TickMsg is sent periodically to update the animation
type TickMsg time.Time

// New creates a new title animator
func New() Animator {
	return Animator{
		animFrame:  0,
		animOffset: 0,
	}
}

// Update updates the animation state when receiving a TickMsg
func (a *Animator) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case TickMsg:
		a.animFrame++
		if a.animFrame >= 30 {
			a.animFrame = 0
			a.animOffset++
		}
		return TickCmd()
	}
	return nil
}

// TickCmd returns a command that sends a TickMsg after a delay
func TickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*80, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Render renders the animated title with a left-to-right color animation
func (a Animator) Render() string {
	titleText := "  cunc  "

	// Color palette for the animation (vibrant colors)
	colors := []lipgloss.Color{
		lipgloss.Color("205"), // Hot Pink
		lipgloss.Color("213"), // Pink
		lipgloss.Color("171"), // Magenta
		lipgloss.Color("135"), // Purple
		lipgloss.Color("99"),  // Light Purple
		lipgloss.Color("63"),  // Blue Purple
		lipgloss.Color("27"),  // Blue
		lipgloss.Color("39"),  // Cyan Blue
		lipgloss.Color("51"),  // Cyan
		lipgloss.Color("50"),  // Teal
		lipgloss.Color("48"),  // Green
		lipgloss.Color("118"), // Light Green
		lipgloss.Color("154"), // Yellow Green
		lipgloss.Color("190"), // Yellow
		lipgloss.Color("226"), // Bright Yellow
		lipgloss.Color("220"), // Gold
		lipgloss.Color("214"), // Orange
		lipgloss.Color("208"), // Dark Orange
		lipgloss.Color("202"), // Red Orange
		lipgloss.Color("196"), // Red
	}

	// Build the title character by character with animated colors
	var coloredTitle strings.Builder
	for i, char := range titleText {
		// Calculate color index based on character position and animation offset
		// The wave moves left to right
		colorIndex := (i + a.animOffset) % len(colors)
		style := lipgloss.NewStyle().
			Bold(true).
			Foreground(colors[colorIndex])
		coloredTitle.WriteString(style.Render(string(char)))
	}

	// Wrap title in a box
	titleBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")). // Nice blue border
		Padding(0, 1).
		Bold(true)

	titleBox := titleBoxStyle.Render(coloredTitle.String())

	// Create version box
	versionBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("99")). // Purple border
		Foreground(lipgloss.Color("99")).
		Padding(0, 1).
		Bold(true)

	versionBox := versionBoxStyle.Render("v" + Version)

	// Join title and version horizontally with a space
	return lipgloss.JoinHorizontal(lipgloss.Top, titleBox, " ", versionBox)
}
