package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)

	allHeader = headerStyle.
			Background(lipgloss.Color("4")).
			Foreground(lipgloss.Color("15")).
			Render

	filteredHeader = headerStyle.
			Background(lipgloss.Color("1")).
			Foreground(lipgloss.Color("15")).
			Render

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
)

type lineMsg string
type eofMsg struct{}

type model struct {
	allVP       viewport.Model
	filteredVP  viewport.Model
	allLines    []string
	filtLines   []string
	terms       []string
	width       int
	height      int
	ready       bool
	done        bool
	followAll   bool
	followFilt  bool
}

func newModel(terms []string) model {
	return model{
		terms:      terms,
		followAll:  true,
		followFilt: true,
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "f":
			m.followAll = !m.followAll
			m.followFilt = !m.followFilt
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.resize()

	case lineMsg:
		line := string(msg)
		m.allLines = append(m.allLines, line)
		m.allVP.SetContent(strings.Join(m.allLines, "\n"))
		if m.followAll {
			m.allVP.GotoBottom()
		}

		matched := false
		for _, term := range m.terms {
			if strings.Contains(line, term) {
				matched = true
				break
			}
		}
		if matched {
			m.filtLines = append(m.filtLines, line)
			m.filteredVP.SetContent(strings.Join(m.filtLines, "\n"))
			if m.followFilt {
				m.filteredVP.GotoBottom()
			}
		}

	case eofMsg:
		m.done = true
	}

	m.allVP, cmd = m.allVP.Update(msg)
	cmds = append(cmds, cmd)
	m.filteredVP, cmd = m.filteredVP.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) resize() model {
	helpHeight := 1
	headerHeight := 1
	borderHeight := 0
	paneHeight := (m.height - helpHeight - (headerHeight+borderHeight)*2) / 2
	if paneHeight < 1 {
		paneHeight = 1
	}
	if !m.ready {
		m.allVP = viewport.New(m.width, paneHeight)
		m.filteredVP = viewport.New(m.width, paneHeight)
		m.ready = true
	} else {
		m.allVP.Width = m.width
		m.allVP.Height = paneHeight
		m.filteredVP.Width = m.width
		m.filteredVP.Height = paneHeight
	}
	return m
}

func (m model) View() string {
	if !m.ready {
		return "waiting for terminal size..."
	}

	status := ""
	if m.done {
		status = " • done"
	}
	followStatus := ""
	if !m.followAll {
		followStatus = " • scroll"
	}

	allTitle := allHeader(fmt.Sprintf(" All Logs (%d lines)%s%s", len(m.allLines), status, followStatus))
	filtTitle := filteredHeader(fmt.Sprintf(" Filtered: [%s] (%d lines)%s%s", strings.Join(m.terms, ", "), len(m.filtLines), status, followStatus))
	help := helpStyle.Render("q: quit • f: toggle follow • ↑↓/pgup/pgdn: scroll")

	return lipgloss.JoinVertical(lipgloss.Left,
		allTitle,
		m.allVP.View(),
		filtTitle,
		m.filteredVP.View(),
		help,
	)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: output-monitor <term> [term...]")
		fmt.Fprintln(os.Stderr, "example: dbt run 2>&1 | output-monitor ERROR WARNING")
		os.Exit(1)
	}

	tty, err := os.Open("/dev/tty")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: could not open /dev/tty:", err)
		os.Exit(1)
	}
	defer tty.Close()

	m := newModel(os.Args[1:])
	p := tea.NewProgram(m, tea.WithInput(tty), tea.WithAltScreen())

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			p.Send(lineMsg(scanner.Text()))
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "read error:", err)
		}
		p.Send(eofMsg{})
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
