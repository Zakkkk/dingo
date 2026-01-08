package main

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/MadAppGang/dingo/pkg/ui"
	"github.com/MadAppGang/dingo/pkg/ui/mascot"
	"github.com/MadAppGang/dingo/pkg/updater"
	"github.com/MadAppGang/dingo/pkg/version"
)

// updateCmd returns the update command for auto-updating Dingo.
func updateCmd() *cobra.Command {
	var (
		force    bool
		noMascot bool
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update Dingo to the latest version",
		Long: `Update downloads and installs the latest version of Dingo from GitHub releases.

The update process:
1. Checks for the latest version
2. Downloads the binary for your platform
3. Backs up the current binary
4. Replaces with the new version
5. Verifies the installation

If the installation fails, Dingo will automatically roll back to the previous version.

Examples:
  dingo update          # Update to latest version
  dingo update --force  # Reinstall even if already up to date`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(force, noMascot)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force update even if already up to date")
	cmd.Flags().BoolVar(&noMascot, "no-mascot", false, "Disable mascot animation")

	return cmd
}

// runUpdate performs the update process.
func runUpdate(force, noMascotFlag bool) error {
	// Auto-detect non-interactive environment
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	showMascot := isTTY && !noMascotFlag

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5AF78E")).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Italic(true)
	highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#56C3F4"))

	// Create build UI for mascot animation
	var buildUI *ui.SimpleBuildUI
	if showMascot {
		buildUI = ui.NewSimpleBuildUI()
		buildUI.Start()
		defer buildUI.Stop()

		// Print header
		fmt.Printf("%s %s\n\n", titleStyle.Render("Dingo"), versionStyle.Render("v"+version.Version))
	}

	// Step 1: Check for updates
	if buildUI != nil {
		buildUI.SetStatus(mascot.StateThinking, "Checking for updates...", "")
	}

	var result *updater.UpdateCheckResult
	var checkErr error

	if showMascot {
		checkErr = runWithUpdateSpinner("Checking", func() error {
			var err error
			result, err = updater.CheckForUpdates(version.Version)
			return err
		})
	} else {
		fmt.Println("Checking for updates...")
		result, checkErr = updater.CheckForUpdates(version.Version)
	}

	if checkErr != nil {
		if buildUI != nil {
			buildUI.SetStatus(mascot.StateFailed, "Check failed!", "")
		}
		return fmt.Errorf("failed to check for updates: %w", checkErr)
	}

	if showMascot {
		fmt.Printf("  %s Check       Done\n\n", successStyle.Render("ok"))
	}

	// Check if update is needed
	if !force && result.Status == updater.UpToDate {
		if buildUI != nil {
			buildUI.SetStatus(mascot.StateSuccess, "Already up to date!", "v"+version.Version)
		} else {
			fmt.Printf("Already up to date (v%s)\n", version.Version)
		}
		return nil
	}

	// Show update info
	fmt.Printf("  %s %s %s %s\n\n",
		dimStyle.Render("Current:"),
		versionStyle.Render("v"+version.Version),
		dimStyle.Render("->"),
		highlightStyle.Render("v"+result.LatestVersion))

	// Check platform support
	if !updater.IsPlatformSupported() {
		osName, arch := updater.GetPlatformInfo()
		return fmt.Errorf("unsupported platform: %s-%s", osName, arch)
	}

	// Step 2: Download new binary
	if buildUI != nil {
		buildUI.SetStatus(mascot.StateCompiling, "Downloading...", "dingo")
	}

	var downloadResult *updater.DownloadResult
	var downloadErr error

	downloadOpts := updater.DownloadOptions{
		Version: result.LatestVersion,
		Binary:  "dingo",
	}

	if showMascot {
		downloadErr = runWithUpdateSpinner("Downloading", func() error {
			var err error
			downloadResult, err = updater.DownloadBinary(downloadOpts)
			return err
		})
	} else {
		fmt.Println("Downloading...")
		downloadResult, downloadErr = updater.DownloadBinary(downloadOpts)
	}

	if downloadErr != nil {
		if buildUI != nil {
			buildUI.SetStatus(mascot.StateFailed, "Download failed!", "")
		}
		return fmt.Errorf("download failed: %w", downloadErr)
	}

	if showMascot {
		fmt.Printf("  %s Download    Done %s\n\n",
			successStyle.Render("ok"),
			dimStyle.Render(fmt.Sprintf("(%s)", formatBytes(downloadResult.Size))))
	}

	// Step 3: Install new binary
	if buildUI != nil {
		buildUI.SetStatus(mascot.StateThinking, "Installing...", "")
	}

	installOpts := updater.InstallOptions{
		TempBinaryPath: downloadResult.TempPath,
	}

	// Check if we need elevated privileges
	exePath, _ := os.Executable()
	if updater.RequiresSudo(exePath) {
		if buildUI != nil {
			buildUI.SetStatus(mascot.StateFailed, "Permission denied!", "")
		}
		// Clean up temp file
		os.Remove(downloadResult.TempPath)
		return fmt.Errorf("permission denied: try running with sudo\n  sudo dingo update")
	}

	var installResult *updater.InstallResult
	var installErr error

	if showMascot {
		installErr = runWithUpdateSpinner("Installing", func() error {
			var err error
			installResult, err = updater.InstallBinary(installOpts)
			return err
		})
	} else {
		fmt.Println("Installing...")
		installResult, installErr = updater.InstallBinary(installOpts)
	}

	if installErr != nil {
		if buildUI != nil {
			buildUI.SetStatus(mascot.StateFailed, "Install failed!", "rolled back")
		}
		return fmt.Errorf("installation failed: %w", installErr)
	}

	if showMascot {
		fmt.Printf("  %s Install     Done\n\n", successStyle.Render("ok"))
	}

	// Success!
	if buildUI != nil {
		buildUI.SetStatus(mascot.StateSuccess, "Update complete!", "v"+result.LatestVersion)
	} else {
		fmt.Printf("\nSuccessfully updated to v%s\n", result.LatestVersion)
		fmt.Printf("  Binary: %s\n", installResult.InstalledPath)
	}

	return nil
}

// runWithUpdateSpinner runs a function while showing a spinner.
func runWithUpdateSpinner(stepName string, fn func() error) error {
	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()

	frameIdx := 0
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	// Mascot frames with spinner eyes
	mascotFrames := [][]string{
		{
			"     ▄▀▄    ▄▀▄       ",
			"     █  ▀▀▀▀▀  █      ",
			"     █  ◜   ◜  █      ",
			"     ▀▄   ▲   ▄▀      ",
			"       ▀▄▄▄▄▄▀        ",
			"      ▄█▀   ▀█▄       ",
			"     ██  ███  ██      ",
			"     ▀█▄▄▀ ▀▄▄█▀      ",
		},
		{
			"     ▄▀▄    ▄▀▄       ",
			"     █  ▀▀▀▀▀  █      ",
			"     █  ◝   ◝  █      ",
			"     ▀▄   ▲   ▄▀      ",
			"       ▀▄▄▄▄▄▀        ",
			"      ▄█▀   ▀█▄       ",
			"     ██  ███  ██      ",
			"     ▀█▄▄▀ ▀▄▄█▀      ",
		},
		{
			"     ▄▀▄    ▄▀▄       ",
			"     █  ▀▀▀▀▀  █      ",
			"     █  ◞   ◞  █      ",
			"     ▀▄   ▲   ▄▀      ",
			"       ▀▄▄▄▄▄▀        ",
			"      ▄█▀   ▀█▄       ",
			"     ██  ███  ██      ",
			"     ▀█▄▄▀ ▀▄▄█▀      ",
		},
		{
			"     ▄▀▄    ▄▀▄       ",
			"     █  ▀▀▀▀▀  █      ",
			"     █  ◟   ◟  █      ",
			"     ▀▄   ▲   ▄▀      ",
			"       ▀▄▄▄▄▄▀        ",
			"      ▄█▀   ▀█▄       ",
			"     ██  ███  ██      ",
			"     ▀█▄▄▀ ▀▄▄█▀      ",
		},
	}

	mascotColor := lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	statusStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Italic(true)
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#45475A"))

	// Status text to show next to mascot
	statusLines := []string{
		"",
		statusStyle.Render(stepName + "..."),
		detailStyle.Render("please wait"),
	}

	// Draw mascot frame
	drawMascotFrame := func(frame []string) {
		// Separator
		fmt.Println(separatorStyle.Render("------------------------------------------------------------"))

		maxLines := len(frame)
		for i := 0; i < maxLines; i++ {
			mascotLine := mascotColor.Render(frame[i])
			statusLine := ""
			if i < len(statusLines) {
				statusLine = "  " + statusLines[i]
			}
			fmt.Printf("%s%s\n", mascotLine, statusLine)
		}
	}

	// Hide cursor
	fmt.Print("\033[?25l")

	// Draw first frame
	drawMascotFrame(mascotFrames[frameIdx])
	mascotHeight := len(mascotFrames[0]) + 1 // +1 for separator

	for {
		select {
		case err := <-done:
			// Move cursor up and clear mascot area
			fmt.Printf("\033[%dA", mascotHeight)
			for i := 0; i < mascotHeight; i++ {
				fmt.Print("\033[2K\n")
			}
			fmt.Printf("\033[%dA", mascotHeight)
			// Show cursor
			fmt.Print("\033[?25h")
			return err
		case <-ticker.C:
			frameIdx = (frameIdx + 1) % len(mascotFrames)
			// Move cursor up to redraw mascot
			fmt.Printf("\033[%dA", mascotHeight)
			drawMascotFrame(mascotFrames[frameIdx])
		}
	}
}

// formatBytes formats a byte count for human readability.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
