package styleguide

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color constants matching Dingo CLI color scheme.
const (
	ColorPurple = "#7D56F4" // Primary purple (titles, mascot)
	ColorGreen  = "#5AF78E" // Success green
	ColorRed    = "#F38BA8" // Error red
	ColorMuted  = "#6C7086" // Muted gray (labels, hints)
	ColorText   = "#CDD6F4" // Text white
	ColorBlue   = "#56C3F4" // Link blue
)

// Formatter handles terminal formatting of styleguide content.
type Formatter struct {
	isTTY   bool
	noColor bool
	styles  StyleSet
}

// StyleSet contains all the lipgloss styles used for formatting.
type StyleSet struct {
	H1         lipgloss.Style
	H2         lipgloss.Style
	H3         lipgloss.Style
	H4         lipgloss.Style
	CodeBlock  lipgloss.Style
	InlineCode lipgloss.Style
	Table      lipgloss.Style
	Normal     lipgloss.Style
	ListItem   lipgloss.Style
	Separator  lipgloss.Style
}

// NewFormatter creates a new formatter with the given settings.
func NewFormatter(isTTY bool, noColor bool) *Formatter {
	f := &Formatter{
		isTTY:   isTTY,
		noColor: noColor,
	}

	if isTTY && !noColor {
		f.styles = StyleSet{
			H1:         lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorPurple)).MarginTop(1).MarginBottom(1),
			H2:         lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorText)).Underline(true).MarginTop(1),
			H3:         lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorText)),
			H4:         lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorMuted)),
			CodeBlock:  lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGreen)),
			InlineCode: lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBlue)),
			Table:      lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)),
			Normal:     lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)),
			ListItem:   lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)),
			Separator:  lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)),
		}
	} else {
		// Plain mode - no styling
		f.styles = StyleSet{
			H1:         lipgloss.NewStyle(),
			H2:         lipgloss.NewStyle(),
			H3:         lipgloss.NewStyle(),
			H4:         lipgloss.NewStyle(),
			CodeBlock:  lipgloss.NewStyle(),
			InlineCode: lipgloss.NewStyle(),
			Table:      lipgloss.NewStyle(),
			Normal:     lipgloss.NewStyle(),
			ListItem:   lipgloss.NewStyle(),
			Separator:  lipgloss.NewStyle(),
		}
	}

	return f
}

// Regex patterns for markdown parsing.
var (
	h1Pattern         = regexp.MustCompile(`^#\s+(.+)$`)
	h2Pattern         = regexp.MustCompile(`^##\s+(.+)$`)
	h3Pattern         = regexp.MustCompile(`^###\s+(.+)$`)
	h4Pattern         = regexp.MustCompile(`^####\s+(.+)$`)
	codeBlockStart    = regexp.MustCompile("^```")
	inlineCodePattern = regexp.MustCompile("`([^`]+)`")
	tableRowPattern   = regexp.MustCompile(`^\|.*\|$`)
	listItemPattern   = regexp.MustCompile(`^(\s*[-*]|\s*\d+\.)\s+`)
	separatorPattern  = regexp.MustCompile(`^---+$`)
)

// FormatMarkdown formats raw markdown content for terminal display.
func (f *Formatter) FormatMarkdown(content string) string {
	if !f.isTTY || f.noColor {
		return content
	}

	lines := strings.Split(content, "\n")
	var result []string
	inCodeBlock := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Handle code blocks
		if codeBlockStart.MatchString(line) {
			inCodeBlock = !inCodeBlock
			if inCodeBlock {
				// Start of code block - skip the ``` line
				continue
			} else {
				// End of code block
				continue
			}
		}

		if inCodeBlock {
			result = append(result, f.styles.CodeBlock.Render(line))
			continue
		}

		// Format headings
		if matches := h1Pattern.FindStringSubmatch(line); matches != nil {
			result = append(result, f.styles.H1.Render(matches[1]))
			continue
		}
		if matches := h2Pattern.FindStringSubmatch(line); matches != nil {
			result = append(result, f.styles.H2.Render(matches[1]))
			continue
		}
		if matches := h3Pattern.FindStringSubmatch(line); matches != nil {
			result = append(result, f.styles.H3.Render(matches[1]))
			continue
		}
		if matches := h4Pattern.FindStringSubmatch(line); matches != nil {
			result = append(result, f.styles.H4.Render(matches[1]))
			continue
		}

		// Handle separator
		if separatorPattern.MatchString(line) {
			result = append(result, f.styles.Separator.Render(strings.Repeat("-", 60)))
			continue
		}

		// Handle table rows
		if tableRowPattern.MatchString(line) {
			result = append(result, f.styles.Table.Render(line))
			continue
		}

		// Handle inline code in normal text
		if strings.Contains(line, "`") {
			line = f.formatInlineCode(line)
		}

		// Handle list items
		if listItemPattern.MatchString(line) {
			result = append(result, f.styles.ListItem.Render(line))
			continue
		}

		// Normal text
		result = append(result, f.styles.Normal.Render(line))
	}

	return strings.Join(result, "\n")
}

// formatInlineCode formats inline code segments within a line.
func (f *Formatter) formatInlineCode(line string) string {
	return inlineCodePattern.ReplaceAllStringFunc(line, func(match string) string {
		// Extract code without backticks
		code := strings.Trim(match, "`")
		return f.styles.InlineCode.Render(code)
	})
}

// FormatSection formats a section with its header and content.
func (f *Formatter) FormatSection(section Section) string {
	var sb strings.Builder

	// Format header based on level
	var headerStyle lipgloss.Style
	switch section.Level {
	case 1:
		headerStyle = f.styles.H1
	case 2:
		headerStyle = f.styles.H2
	case 3:
		headerStyle = f.styles.H3
	default:
		headerStyle = f.styles.H4
	}

	if f.isTTY && !f.noColor {
		sb.WriteString(headerStyle.Render(section.Title))
	} else {
		sb.WriteString(strings.Repeat("#", section.Level) + " " + section.Title)
	}
	sb.WriteString("\n\n")

	// Format content
	sb.WriteString(f.FormatMarkdown(section.Content))

	return sb.String()
}

// FormatSectionList formats a list of sections for the --list flag.
func (f *Formatter) FormatSectionList(sections []Section) string {
	var sb strings.Builder

	if f.isTTY && !f.noColor {
		sb.WriteString(f.styles.H1.Render("Available Sections"))
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# Available Sections\n\n")
	}

	for _, s := range sections {
		indent := strings.Repeat("  ", s.Level-1)
		if f.isTTY && !f.noColor {
			switch s.Level {
			case 1:
				sb.WriteString(f.styles.H2.Render(s.Title))
			case 2:
				sb.WriteString(indent + f.styles.H3.Render(s.Title))
			default:
				sb.WriteString(indent + f.styles.Normal.Render(s.Title))
			}
		} else {
			sb.WriteString(indent + s.Title)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
