package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ms/amplifier-app-loom/internal/config"
)

// bundleCmd is the parent command for Amplifier app bundle management.
var bundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Manage Amplifier app bundles",
	Long: `Browse the Amplifier registry and manage app bundles.

App bundles extend every Amplifier session with additional tools, agents,
and behaviors. Use 'bundle add' to install from the registry, 'bundle list'
to see what's installed, and 'bundle remove' to uninstall.

To register loom itself as an Amplifier app bundle (so the loom bundle is
available in every session), run:
  loom bundle install`,
}

// ── bundle install ────────────────────────────────────────────────────────────

var bundleInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Register loom as an Amplifier app bundle",
	Long: `Installs the loom bundle globally so the amplifier CLI composes it
into every session. This runs:
  amplifier bundle add git+https://github.com/kenotron-ms/amplifier-app-loom@main --app`,
	RunE: func(cmd *cobra.Command, args []string) error {
		spec := "git+https://github.com/kenotron-ms/amplifier-app-loom@main"
		fmt.Printf("Installing loom as an Amplifier app bundle...\n")
		if err := runAmplifierBundleAdd(spec); err != nil {
			return fmt.Errorf("amplifier bundle add failed: %w\n\nMake sure `amplifier` is installed and try again.", err)
		}
		fmt.Printf("✓ loom is now an Amplifier app bundle\n")
		fmt.Printf("  Compose it into any bundle with:\n")
		fmt.Printf("  amplifier bundle add %s --app\n", spec)
		return nil
	},
}

// ── bundle list ───────────────────────────────────────────────────────────────

var bundleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed app bundles",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/bundles", port))
		if err != nil {
			return fmt.Errorf("daemon not reachable: %w", err)
		}
		defer resp.Body.Close()

		var bundles []config.AppBundle
		if err := json.NewDecoder(resp.Body).Decode(&bundles); err != nil {
			return err
		}
		if len(bundles) == 0 {
			fmt.Println("No app bundles installed.")
			fmt.Println("Browse the registry at http://localhost:7700 or run: loom bundle add <spec>")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tENABLED\tINSTALL SPEC")
		for _, b := range bundles {
			enabled := "yes"
			if !b.Enabled {
				enabled = "no"
			}
			name := b.Name
			if name == "" {
				name = b.ID
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", b.ID, name, enabled, b.InstallSpec)
		}
		return w.Flush()
	},
}

// ── bundle add ────────────────────────────────────────────────────────────────

var bundleAddCmd = &cobra.Command{
	Use:   "add <install-spec>",
	Short: "Add an app bundle from a spec or registry install string",
	Example: `  # From registry (paste the install command from the registry):
  loom bundle add "amplifier bundle add superpowers"
  loom bundle add "amplifier bundle add git+https://github.com/…@main"

  # Or just the spec after "amplifier bundle add":
  loom bundle add superpowers
  loom bundle add git+https://github.com/…@main`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		raw := strings.TrimSpace(args[0])

		// Accept "amplifier bundle add <spec>" or just "<spec>"
		spec := raw
		if strings.HasPrefix(raw, "amplifier bundle add ") {
			spec = strings.TrimPrefix(raw, "amplifier bundle add ")
		}
		spec = strings.TrimSpace(spec)
		if spec == "" {
			return fmt.Errorf("install spec is empty")
		}

		// Derive a display ID from the spec
		id := specToID(spec)

		body, _ := json.Marshal(map[string]string{
			"id":          id,
			"installSpec": spec,
			"name":        id,
		})

		resp, err := http.Post(
			fmt.Sprintf("http://localhost:%d/api/bundles", port),
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			return fmt.Errorf("daemon not reachable: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusConflict {
			fmt.Printf("Bundle '%s' is already installed.\n", id)
			return nil
		}
		if resp.StatusCode >= 400 {
			var e map[string]string
			json.NewDecoder(resp.Body).Decode(&e)
			return fmt.Errorf("error: %s", e["error"])
		}

		fmt.Printf("✓ Bundle added: %s\n", spec)
		fmt.Printf("  Manage bundles at http://localhost:%d (Bundles tab)\n", port)
		return nil
	},
}

// ── bundle remove ─────────────────────────────────────────────────────────────

var bundleRemoveCmd = &cobra.Command{
	Use:     "remove <id>",
	Aliases: []string{"rm", "uninstall"},
	Short:   "Remove an installed app bundle by ID",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		id := args[0]

		req, _ := http.NewRequest(http.MethodDelete,
			fmt.Sprintf("http://localhost:%d/api/bundles/%s", port, id), nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("daemon not reachable: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("no bundle found with id '%s'", id)
		}
		if resp.StatusCode >= 400 {
			var e map[string]string
			json.NewDecoder(resp.Body).Decode(&e)
			return fmt.Errorf("error: %s", e["error"])
		}
		fmt.Printf("✓ Bundle removed: %s\n", id)
		return nil
	},
}

// ── helpers ───────────────────────────────────────────────────────────────────

// specToID derives a short stable ID from a bundle install spec.
//   "git+https://github.com/microsoft/amplifier-bundle-superpowers@main"  → "superpowers"
//   "superpowers"                                                           → "superpowers"
//   "foundation:foundation-expert"                                          → "foundation-expert"
func specToID(spec string) string {
	// git URL: extract repo name without amplifier-bundle- prefix
	if strings.Contains(spec, "github.com/") {
		parts := strings.Split(spec, "/")
		repo := parts[len(parts)-1]
		repo = strings.TrimSuffix(repo, "@main")
		repo = strings.TrimSuffix(repo, ".git")
		if strings.HasPrefix(repo, "amplifier-bundle-") {
			repo = strings.TrimPrefix(repo, "amplifier-bundle-")
		} else if strings.HasPrefix(repo, "amplifier-module-") {
			repo = strings.TrimPrefix(repo, "amplifier-module-")
		}
		return repo
	}
	// namespace:name → name
	if strings.Contains(spec, ":") {
		parts := strings.SplitN(spec, ":", 2)
		return parts[1]
	}
	return spec
}

// runAmplifierBundleAdd calls `amplifier bundle add <spec> --app`.
func runAmplifierBundleAdd(spec string) error {
	ampBin := findAmplifierBin()
	cmd := exec.Command(ampBin, "bundle", "add", spec, "--app")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// findAmplifierBin locates the amplifier binary, checking PATH and common install
// locations. Mirrors api.resolveAmplifier() but lives in the cli package.
func findAmplifierBin() string {
	if p, err := exec.LookPath("amplifier"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		home + "/.local/bin/amplifier",
		"/usr/local/bin/amplifier",
		"/opt/homebrew/bin/amplifier",
		home + "/go/bin/amplifier",
	}
	for _, p := range candidates {
		if _, err := exec.LookPath(p); err == nil {
			return p
		}
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return "amplifier"
}

func init() {
	for _, cmd := range []*cobra.Command{
		bundleListCmd, bundleAddCmd, bundleRemoveCmd, bundleInstallCmd,
	} {
		cmd.Flags().Int("port", config.DefaultPort, "Daemon port")
	}

	bundleCmd.AddCommand(bundleListCmd, bundleAddCmd, bundleRemoveCmd, bundleInstallCmd)
}
