package title

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// version is the current version of the application
// update this when releasing new versions, or don't
const Version = "0.1.0"

// animator handles the animated title state and rendering, because drama
type Animator struct {
	anim_frame  int
	anim_offset int
}

// tickmsg is sent periodically to update the animation, like clockwork
type TickMsg time.Time

// new creates a new title animator, fresh and shiny
func New() Animator {
	return Animator{
		anim_frame:  0,
		anim_offset: 0,
	}
}

// update updates the animation state when receiving a tickmsg, keep up
func (a *Animator) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case TickMsg:
		a.anim_frame++
		if a.anim_frame >= 30 {
			a.anim_frame = 0
			a.anim_offset++
		}
		return TickCmd()
	}
	return nil
}

// tickcmd returns a command that sends a tickmsg after a delay
func TickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*80, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// render renders the animated title with a left-to-right color animation
func (a Animator) Render() string {
	title_text := "  cunc  "

	// color palette for the animation, because why not
	colors := []lipgloss.Color{
		lipgloss.Color("205"),
		lipgloss.Color("213"),
		lipgloss.Color("171"),
		lipgloss.Color("135"),
		lipgloss.Color("99"),
		lipgloss.Color("63"),
		lipgloss.Color("27"),
		lipgloss.Color("39"),
		lipgloss.Color("51"),
		lipgloss.Color("50"),
		lipgloss.Color("48"),
		lipgloss.Color("118"),
		lipgloss.Color("154"),
		lipgloss.Color("190"),
		lipgloss.Color("226"),
		lipgloss.Color("220"),
		lipgloss.Color("214"),
		lipgloss.Color("208"),
		lipgloss.Color("202"),
		lipgloss.Color("196"),
	}

	// build the title character by character with animated colors
	var colored_title strings.Builder
	for i, char := range title_text {
		// calculate color index based on position and animation offset
		// the wave moves left to right, because waves do that
		color_index := (i + a.anim_offset) % len(colors)
		style := lipgloss.NewStyle().
			Bold(true).
			Foreground(colors[color_index])
		colored_title.WriteString(style.Render(string(char)))
	}

	// wrap title in a box, so it feels important
	title_box_style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1).
		Bold(true)

	title_box := title_box_style.Render(colored_title.String())

	// create version box, because version numbers deserve attention
	version_box_style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("99")).
		Foreground(lipgloss.Color("99")).
		Padding(0, 1).
		Bold(true)

	version_box := version_box_style.Render("v" + Version)

	// join title and version horizontally with a space, simple stuff
	return lipgloss.JoinHorizontal(lipgloss.Top, title_box, " ", version_box)
}
