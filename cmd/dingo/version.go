package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/MadAppGang/dingo/pkg/ui/mascot"
	"github.com/MadAppGang/dingo/pkg/updater"
	"github.com/MadAppGang/dingo/pkg/version"
)

// newVersionCmd returns the enhanced version command with mascot and update check.
func newVersionCmd() *cobra.Command {
	var noMascot bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version and check for updates",
		Long: `Print the Dingo version number and check GitHub for updates.

If a newer version is available, you'll see a notification with
instructions to run 'dingo update' to upgrade.

Examples:
  dingo version              # Show version with mascot
  dingo version --no-mascot  # Show version without mascot`,
		Run: func(cmd *cobra.Command, args []string) {
			runVersionCommand(noMascot)
		},
	}

	cmd.Flags().BoolVar(&noMascot, "no-mascot", false, "Disable mascot display")

	return cmd
}

// runVersionCommand displays version info with mascot and checks for updates.
func runVersionCommand(noMascotFlag bool) {
	// Auto-detect non-interactive environment
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	showMascot := isTTY && !noMascotFlag

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5AF78E"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CDD6F4"))
	linkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#56C3F4"))
	updateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F7DC6F")).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Italic(true)

	// Version info lines
	infoLines := []string{
		"",
		titleStyle.Render("Dingo") + " " + versionStyle.Render("v"+version.Version),
		"",
		labelStyle.Render("Runtime:") + "  " + valueStyle.Render("Go"),
		labelStyle.Render("Website:") + "  " + linkStyle.Render("https://dingo-lang.org"),
		labelStyle.Render("GitHub:") + "   " + linkStyle.Render("github.com/MadAppGang/dingo"),
	}

	// Check for updates in background
	var updateResult *updater.UpdateCheckResult
	var updateErr error
	var updateWg sync.WaitGroup

	updateWg.Add(1)
	go func() {
		defer updateWg.Done()
		updateResult, updateErr = updater.CheckForUpdates(version.Version)
	}()

	if showMascot {
		// Print mascot with version info side by side
		printMascotWithInfo(infoLines)

		// Wait for update check to complete
		updateWg.Wait()

		// Show update notification if available
		if updateErr == nil && updateResult.Status == updater.UpdateAvailable {
			fmt.Println()
			fmt.Println(updateStyle.Render("  Update available: v" + updateResult.LatestVersion))
			fmt.Println(hintStyle.Render("  Run 'dingo update' to upgrade"))
		}
	} else {
		// Simple text output
		fmt.Println()
		fmt.Println(titleStyle.Render("Dingo"))
		fmt.Println()
		fmt.Printf("  %s %s\n", labelStyle.Render("Version:"), versionStyle.Render(version.Version))
		fmt.Printf("  %s %s\n", labelStyle.Render("Runtime:"), valueStyle.Render("Go"))
		fmt.Printf("  %s %s\n", labelStyle.Render("Website:"), linkStyle.Render("https://dingo-lang.org"))
		fmt.Printf("  %s %s\n", labelStyle.Render("GitHub:"), linkStyle.Render("github.com/MadAppGang/dingo"))
		fmt.Println()

		// Wait for update check
		updateWg.Wait()

		if updateErr == nil && updateResult.Status == updater.UpdateAvailable {
			fmt.Println(updateStyle.Render("Update available: v" + updateResult.LatestVersion))
			fmt.Println(hintStyle.Render("Run 'dingo update' to upgrade"))
			fmt.Println()
		}
	}
}

// printMascotWithInfo prints the mascot with version info on the right.
func printMascotWithInfo(infoLines []string) {
	// Mascot frame (help state - friendly pose)
	mascotFrame := []string{
		"     ▄▀▄    ▄▀▄       ",
		"     █  ▀▀▀▀▀  █      ",
		"     █  ^   ^  █      ",
		"     ▀▄   ▲   ▄▀      ",
		"       ▀▄▄▄▄▄▀        ",
		"      ▄█▀   ▀█▄       ",
		"     ██  ███  ██      ",
		"     ▀█▄▄▀ ▀▄▄█▀      ",
	}

	// Apply mascot color
	mascotColor := lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))

	// Print separator
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#45475A"))
	fmt.Println(separatorStyle.Render(strings.Repeat("-", 60)))

	// Print side by side
	maxLines := len(mascotFrame)
	if len(infoLines) > maxLines {
		maxLines = len(infoLines)
	}

	for i := 0; i < maxLines; i++ {
		mascotLine := ""
		if i < len(mascotFrame) {
			mascotLine = mascotColor.Render(mascotFrame[i])
		} else {
			mascotLine = strings.Repeat(" ", 22) // Mascot width
		}

		infoLine := ""
		if i < len(infoLines) {
			infoLine = infoLines[i]
		}

		fmt.Printf("%s%s\n", mascotLine, infoLine)
	}
}

// versionCmdWithMascot creates a simple version command that shows version with mascot.
// This is used if the enhanced version check should be disabled.
func versionCmdSimple() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Dingo",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("dingo version %s\n", version.Version)
		},
	}
}

// mascotFrameWithState returns a mascot frame for a given state.
func mascotFrameWithState(state mascot.MascotState) []string {
	m := mascot.New(mascot.WithInitialState(state))
	return m.Render()
}

// runVersionWithAnimation runs a brief animation before showing version.
// This is a placeholder for future enhancement.
func runVersionWithAnimation() {
	// Animation frames (thinking -> success)
	frames := []struct {
		delay time.Duration
		state mascot.MascotState
	}{
		{100 * time.Millisecond, mascot.StateThinking},
		{100 * time.Millisecond, mascot.StateThinking},
		{100 * time.Millisecond, mascot.StateThinking},
		{200 * time.Millisecond, mascot.StateSuccess},
	}

	colorScheme := mascot.DefaultColorScheme

	// Hide cursor
	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h")

	lastHeight := 0

	for _, f := range frames {
		// Clear previous frame
		if lastHeight > 0 {
			fmt.Printf("\033[%dA", lastHeight)
			for i := 0; i < lastHeight; i++ {
				fmt.Print("\033[2K\n")
			}
			fmt.Printf("\033[%dA", lastHeight)
		}

		// Get and print frame
		frame := mascotFrameWithState(f.state)
		for _, line := range frame {
			fmt.Println(colorScheme.ApplyBodyColor(line))
		}
		lastHeight = len(frame)

		time.Sleep(f.delay)
	}

	// Clear final frame
	if lastHeight > 0 {
		fmt.Printf("\033[%dA", lastHeight)
		for i := 0; i < lastHeight; i++ {
			fmt.Print("\033[2K\n")
		}
		fmt.Printf("\033[%dA", lastHeight)
	}
}
