package cli

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/ms/amplifier-app-loom/internal/api"
	"github.com/ms/amplifier-app-loom/internal/updater"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update loom to the latest release",
	Long: `Download the latest release from GitHub, verify its checksum, kill all
running loom processes, install the update, and relaunch.

On macOS: downloads the signed DMG, installs Loom.app to /Applications, and
launches the tray. On Linux/Windows: atomically swaps the binary and restarts
the service.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Current version: v%s\n", api.Version)

		// Kill every loom process (service + tray + any other subcommands)
		// before touching anything on disk. The service is also uninstalled
		// so the service manager won't restart it mid-update.
		fmt.Println("Stopping all loom processes…")
		updater.KillAllLoomProcesses()

		fmt.Println("Checking for updates…")

		u := updater.New(api.Version, func(s updater.State, ver string) {
			switch s {
			case updater.StateChecking:
				// already printed above
			case updater.StateDownloading:
				fmt.Printf("New version v%s found — downloading…\n", ver)
			case updater.StateReady:
				fmt.Printf("Download complete — applying update to v%s…\n", ver)
			case updater.StateApplying:
				if runtime.GOOS == "darwin" {
					fmt.Println("Installing Loom.app to /Applications…")
				} else {
					fmt.Println("Swapping binary and reinstalling service…")
				}
			case updater.StateFailed:
				// error is returned below
			}
		})

		if err := u.CheckAndStage(context.Background()); err != nil {
			return fmt.Errorf("update check/download: %w", err)
		}

		switch u.State() {
		case updater.StateUpToDate:
			fmt.Printf("Already up to date (v%s).\n", api.Version)
			return nil

		case updater.StateReady:
			if runtime.GOOS == "darwin" {
				// macOS: mount DMG → install to /Applications → launch tray.
				if err := u.ApplyDMG(); err != nil {
					return fmt.Errorf("apply DMG update: %w", err)
				}
				fmt.Printf("\n✓ Updated to v%s — launching Loom…\n", u.LatestVersion())
				if err := exec.Command("open", "-a", "Loom").Start(); err != nil {
					fmt.Printf("  Note: could not launch tray automatically: %v\n", err)
					fmt.Println("  Run: open -a Loom")
				}
			} else {
				// Linux / Windows: stop service → swap binary → reinstall service.
				// The daemon is restarted by the service manager automatically.
				if _, err := u.Apply(); err != nil {
					return fmt.Errorf("apply update: %w", err)
				}
				fmt.Printf("\n✓ Updated to v%s. The daemon has been restarted.\n", u.LatestVersion())
			}

			// Re-register the Amplifier bundle so the updated version is active.
			installAmplifierBundleIfDetected()

			return nil

		default:
			return fmt.Errorf("unexpected updater state: %s", u.State())
		}
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
