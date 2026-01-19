// Package styleguide provides access to the Dingo style guide content.
package styleguide

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"
)

//go:embed content/DINGO_STYLE_GUIDE.md
var StyleGuideContent string

// Section represents a parsed section of the styleguide.
type Section struct {
	Title   string
	Content string
	Level   int // Heading level (1-4)
}

// headingRegex matches markdown headings.
var headingRegex = regexp.MustCompile(`^(#{1,4})\s+(.+)$`)

// ParseSections parses the styleguide into sections.
func ParseSections() []Section {
	lines := strings.Split(StyleGuideContent, "\n")
	var sections []Section
	var currentSection *Section
	var contentLines []string

	for _, line := range lines {
		matches := headingRegex.FindStringSubmatch(line)
		if matches != nil {
			// Save previous section
			if currentSection != nil {
				currentSection.Content = strings.TrimSpace(strings.Join(contentLines, "\n"))
				sections = append(sections, *currentSection)
			}

			// Start new section
			level := len(matches[1])
			currentSection = &Section{
				Title: matches[2],
				Level: level,
			}
			contentLines = []string{}
		} else if currentSection != nil {
			contentLines = append(contentLines, line)
		}
	}

	// Save last section
	if currentSection != nil {
		currentSection.Content = strings.TrimSpace(strings.Join(contentLines, "\n"))
		sections = append(sections, *currentSection)
	}

	return sections
}

// GetSection returns sections matching the given title (case-insensitive substring match).
// Returns all matching sections.
func GetSection(title string) ([]Section, error) {
	sections := ParseSections()
	titleLower := strings.ToLower(title)

	var matches []Section
	for _, s := range sections {
		if strings.Contains(strings.ToLower(s.Title), titleLower) {
			matches = append(matches, s)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no section found matching %q", title)
	}

	return matches, nil
}

// GetQuickReference returns "Part 1: Quick Reference" content.
func GetQuickReference() string {
	sections := ParseSections()

	// Find the Part 1 section and collect all its subsections
	var quickRefContent strings.Builder
	inQuickRef := false

	for _, s := range sections {
		if strings.Contains(s.Title, "Part 1: Quick Reference") {
			inQuickRef = true
			quickRefContent.WriteString(formatSectionHeader(s))
			quickRefContent.WriteString(s.Content)
			quickRefContent.WriteString("\n\n")
			continue
		}

		// Stop when we hit Part 2
		if strings.Contains(s.Title, "Part 2") {
			break
		}

		if inQuickRef {
			quickRefContent.WriteString(formatSectionHeader(s))
			quickRefContent.WriteString(s.Content)
			quickRefContent.WriteString("\n\n")
		}
	}

	return strings.TrimSpace(quickRefContent.String())
}

// ListSections returns a list of all section titles with their levels.
func ListSections() []Section {
	sections := ParseSections()
	// Return just titles and levels, no content
	result := make([]Section, len(sections))
	for i, s := range sections {
		result[i] = Section{
			Title: s.Title,
			Level: s.Level,
		}
	}
	return result
}

// formatSectionHeader creates a markdown header for a section.
func formatSectionHeader(s Section) string {
	return strings.Repeat("#", s.Level) + " " + s.Title + "\n\n"
}

// GetFullContent returns the complete styleguide content.
func GetFullContent() string {
	return StyleGuideContent
}
