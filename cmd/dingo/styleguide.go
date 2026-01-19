package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/MadAppGang/dingo/pkg/styleguide"
)

// styleguideCmd returns the "dingo styleguide" command.
func styleguideCmd() *cobra.Command {
	var (
		section string
		quick   bool
		list    bool
		noColor bool
	)

	cmd := &cobra.Command{
		Use:   "styleguide",
		Short: "Print Dingo style guide and best practices",
		Long: `Print the Dingo style guide with best practices for writing idiomatic Dingo code.

This command is especially useful for AI coding agents to understand Dingo patterns.

Examples:
  dingo styleguide                             # Print full guide
  dingo styleguide --quick                     # Print quick reference only
  dingo styleguide --section "Error Propagation"
  dingo styleguide --section "Result"          # Partial match works
  dingo styleguide --list                      # List available sections
  dingo styleguide --no-color                  # Plain text output`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStyleguide(section, quick, list, noColor)
		},
	}

	cmd.Flags().StringVar(&section, "section", "", "Print specific section by title (case-insensitive, partial match)")
	cmd.Flags().BoolVar(&quick, "quick", false, "Print quick reference only (Part 1)")
	cmd.Flags().BoolVar(&list, "list", false, "List available sections")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colors and styling")

	return cmd
}

// runStyleguide executes the styleguide command logic.
func runStyleguide(sectionName string, quick bool, list bool, noColor bool) error {
	// Detect TTY for styling
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	formatter := styleguide.NewFormatter(isTTY, noColor)

	// Handle --list flag
	if list {
		sections := styleguide.ListSections()
		output := formatter.FormatSectionList(sections)
		fmt.Println(output)
		return nil
	}

	// Handle --quick flag
	if quick {
		content := styleguide.GetQuickReference()
		output := formatter.FormatMarkdown(content)
		fmt.Println(output)
		return nil
	}

	// Handle --section flag
	if sectionName != "" {
		sections, err := styleguide.GetSection(sectionName)
		if err != nil {
			// Show helpful message with available sections
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			fmt.Fprintln(os.Stderr, "Available sections (use --list to see all):")
			allSections := styleguide.ListSections()
			for _, s := range allSections {
				if s.Level <= 2 {
					indent := strings.Repeat("  ", s.Level-1)
					fmt.Fprintf(os.Stderr, "%s- %s\n", indent, s.Title)
				}
			}
			return nil // Already printed helpful error message
		}

		// Print matching sections
		var sb strings.Builder
		for i, s := range sections {
			if i > 0 {
				sb.WriteString("\n---\n\n")
			}
			sb.WriteString(formatter.FormatSection(s))
		}
		fmt.Println(sb.String())
		return nil
	}

	// Default: print full styleguide
	content := styleguide.GetFullContent()
	output := formatter.FormatMarkdown(content)
	fmt.Println(output)
	return nil
}
