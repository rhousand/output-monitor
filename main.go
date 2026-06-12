package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1)

	allHeader = headerStyle.
			Background(lipgloss.Color("4")).
			Foreground(lipgloss.Color("15")).
			Render

	filteredHeader = headerStyle.
			Background(lipgloss.Color("1")).
			Foreground(lipgloss.Color("15")).
			Render

	helpStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	contextStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	// per-term highlight colors: bright red, yellow, cyan, green, magenta, blue
	termPalette = []lipgloss.Color{"9", "11", "14", "10", "13", "12"}
)

type lineMsg string
type eofMsg struct{}

type pattern struct {
	raw string
	re  *regexp.Regexp // nil in plain-string mode
}

func compilePatterns(terms []string, useRegex, ignoreCase bool) ([]pattern, error) {
	patterns := make([]pattern, len(terms))
	for i, term := range terms {
		p := pattern{raw: term}
		if useRegex {
			reStr := term
			if ignoreCase {
				reStr = "(?i)" + reStr
			}
			re, err := regexp.Compile(reStr)
			if err != nil {
				return nil, fmt.Errorf("invalid regex %q: %w", term, err)
			}
			p.re = re
		}
		patterns[i] = p
	}
	return patterns, nil
}

func matchLine(line string, patterns []pattern, ignoreCase bool) bool {
	for _, p := range patterns {
		if p.re != nil {
			if p.re.MatchString(line) {
				return true
			}
		} else {
			hay, needle := line, p.raw
			if ignoreCase {
				hay = strings.ToLower(line)
				needle = strings.ToLower(p.raw)
			}
			if strings.Contains(hay, needle) {
				return true
			}
		}
	}
	return false
}

// highlightLine colors all matched terms in the line using per-term palette colors.
func highlightLine(line string, patterns []pattern, colors []lipgloss.Color, ignoreCase bool) string {
	var sb strings.Builder
	pos := 0
	for pos < len(line) {
		earliest, earliestEnd, earliestColor := -1, -1, 0

		for i, p := range patterns {
			var idx, end int
			if p.re != nil {
				loc := p.re.FindStringIndex(line[pos:])
				if loc == nil {
					continue
				}
				idx, end = loc[0], loc[1]
			} else {
				hay, needle := line[pos:], p.raw
				if ignoreCase {
					hay = strings.ToLower(line[pos:])
					needle = strings.ToLower(p.raw)
				}
				idx = strings.Index(hay, needle)
				if idx < 0 {
					continue
				}
				end = idx + len(p.raw)
			}
			if earliest == -1 || idx < earliest {
				earliest, earliestEnd, earliestColor = idx, end, i
			}
		}

		if earliest == -1 {
			sb.WriteString(line[pos:])
			break
		}

		sb.WriteString(line[pos : pos+earliest])
		sb.WriteString(lipgloss.NewStyle().
			Foreground(colors[earliestColor%len(colors)]).
			Bold(true).
			Render(line[pos+earliest : pos+earliestEnd]))
		pos += earliestEnd
	}
	return sb.String()
}

type model struct {
	allVP        viewport.Model
	filteredVP   viewport.Model
	allLines     []string
	filtLines    []string
	terms        []string
	patterns     []pattern
	termColors   []lipgloss.Color
	ignoreCase   bool
	useRegex     bool
	timestamps   bool
	contextN     int
	bell         bool
	tty          *os.File
	width        int
	height       int
	ready        bool
	done         bool
	followAll    bool
	followFilt   bool
	startTime    time.Time
	contextBuf   []string // sliding window of last contextN display lines
	contextAfter int      // lines remaining to show after current match
	inMatchGroup bool     // true while in a context window
}

func newModel(terms []string, patterns []pattern, ignoreCase, useRegex, timestamps bool, contextN int, bell bool, tty *os.File) model {
	colors := make([]lipgloss.Color, len(terms))
	for i := range terms {
		colors[i] = termPalette[i%len(termPalette)]
	}
	return model{
		terms:      terms,
		patterns:   patterns,
		termColors: colors,
		ignoreCase: ignoreCase,
		useRegex:   useRegex,
		timestamps: timestamps,
		contextN:   contextN,
		bell:       bell,
		tty:        tty,
		followAll:  true,
		followFilt: true,
		startTime:  time.Now(),
	}
}

func (m model) Init() tea.Cmd { return nil }

func bellCmd(tty *os.File) tea.Cmd {
	return func() tea.Msg {
		fmt.Fprint(tty, "\a")
		return nil
	}
}

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
		raw := string(msg)

		display := raw
		if m.timestamps {
			display = time.Now().Format("15:04:05.000") + " " + raw
		}

		m.allLines = append(m.allLines, display)
		m.allVP.SetContent(strings.Join(m.allLines, "\n"))
		if m.followAll {
			m.allVP.GotoBottom()
		}

		prevFiltLen := len(m.filtLines)
		matched := matchLine(raw, m.patterns, m.ignoreCase)

		if matched {
			if m.contextN > 0 && len(m.filtLines) > 0 && !m.inMatchGroup {
				m.filtLines = append(m.filtLines, separatorStyle.Render("---"))
				for _, cl := range m.contextBuf {
					m.filtLines = append(m.filtLines, contextStyle.Render(cl))
				}
			}
			m.filtLines = append(m.filtLines, highlightLine(display, m.patterns, m.termColors, m.ignoreCase))
			m.contextAfter = m.contextN
			m.inMatchGroup = m.contextN > 0

			if m.bell {
				cmds = append(cmds, bellCmd(m.tty))
			}
		} else if m.contextAfter > 0 {
			m.filtLines = append(m.filtLines, contextStyle.Render(display))
			m.contextAfter--
			if m.contextAfter == 0 {
				m.inMatchGroup = false
			}
		} else {
			m.inMatchGroup = false
		}

		if len(m.filtLines) != prevFiltLen {
			m.filteredVP.SetContent(strings.Join(m.filtLines, "\n"))
			if m.followFilt {
				m.filteredVP.GotoBottom()
			}
		}

		if m.contextN > 0 {
			m.contextBuf = append(m.contextBuf, display)
			if len(m.contextBuf) > m.contextN {
				m.contextBuf = m.contextBuf[1:]
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
	paneHeight := (m.height - helpHeight - headerHeight*2) / 2
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

	rateStr := ""
	if elapsed := time.Since(m.startTime).Seconds(); elapsed > 0.5 {
		rateStr = fmt.Sprintf(" • %.0f/s", float64(len(m.allLines))/elapsed)
	}

	allTitle := allHeader(fmt.Sprintf(
		" All Logs (%d lines)%s%s%s",
		len(m.allLines), rateStr, status, followStatus,
	))

	flags := ""
	if m.ignoreCase {
		flags += " -i"
	}
	if m.useRegex {
		flags += " -r"
	}
	if m.contextN > 0 {
		flags += fmt.Sprintf(" -C%d", m.contextN)
	}
	filtTitle := filteredHeader(fmt.Sprintf(
		" Filtered: [%s]%s (%d lines)%s%s",
		strings.Join(m.terms, ", "), flags, len(m.filtLines), status, followStatus,
	))

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
	ignoreCase := flag.Bool("i", false, "case-insensitive matching")
	useRegex := flag.Bool("r", false, "treat terms as regular expressions")
	timestamps := flag.Bool("t", false, "prefix each line with timestamp")
	contextN := flag.Int("C", 0, "lines of context around each match")
	bell := flag.Bool("b", false, "ring terminal bell on match")
	outFile := flag.String("o", "", "write all output to file")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: output-monitor [flags] <term> [term...]")
		fmt.Fprintln(os.Stderr, "example: dbt run 2>&1 | output-monitor -i -t -C 2 error warning")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	patterns, err := compilePatterns(flag.Args(), *useRegex, *ignoreCase)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	var output *os.File
	if *outFile != "" {
		output, err = os.Create(*outFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error opening output file:", err)
			os.Exit(1)
		}
		defer output.Close()
	}

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: could not open /dev/tty:", err)
		os.Exit(1)
	}
	defer tty.Close()

	m := newModel(flag.Args(), patterns, *ignoreCase, *useRegex, *timestamps, *contextN, *bell, tty)
	p := tea.NewProgram(m, tea.WithInput(tty), tea.WithAltScreen())

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			if output != nil {
				fmt.Fprintln(output, line)
			}
			p.Send(lineMsg(line))
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
