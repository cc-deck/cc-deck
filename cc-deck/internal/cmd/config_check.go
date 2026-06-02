package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/config"
)

// NewCheckCmd creates the 'config check' subcommand.
func NewCheckCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Validate configuration file",
		Long: `Validate the cc-deck configuration file for common issues.

Checks badge icon widths, profile required fields, voice parameter ranges,
and badge rule structure. Reports findings with severity, category, and
fix suggestions.

Exit code 0 when no errors are found (warnings only or clean).
Exit code 1 when any error is found.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(gf)
		},
	}
}

func runCheck(gf *GlobalFlags) error {
	cfg, err := config.Load(gf.ConfigFile)
	if err != nil {
		return err
	}

	findings := cfg.Validate()
	if len(findings) == 0 {
		fmt.Println("config: no issues found")
		return nil
	}

	// Group findings by category
	grouped := map[config.Category][]config.Finding{}
	for _, f := range findings {
		grouped[f.Category] = append(grouped[f.Category], f)
	}

	// Print in stable category order
	categories := make([]config.Category, 0, len(grouped))
	for cat := range grouped {
		categories = append(categories, cat)
	}
	sort.Slice(categories, func(i, j int) bool {
		return string(categories[i]) < string(categories[j])
	})

	var errors int
	for _, cat := range categories {
		fmt.Printf("\n[%s]\n", cat)
		for _, f := range grouped[cat] {
			marker := "warning"
			if f.Severity == config.SeverityError {
				marker = "error"
				errors++
			}
			fmt.Printf("  %s: %s\n", marker, f.Message)
			if f.Suggestion != "" {
				fmt.Printf("    fix: %s\n", f.Suggestion)
			}
		}
	}

	fmt.Println()
	if errors > 0 {
		return fmt.Errorf("%d error(s) found", errors)
	}

	return nil
}
