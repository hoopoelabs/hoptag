package cmd

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hoopoelabs/hoptag/internal/hcl"
	"github.com/hoopoelabs/hoptag/internal/models"
	"github.com/hoopoelabs/hoptag/internal/providers"
	"github.com/hoopoelabs/hoptag/internal/validator"
	"github.com/spf13/cobra"
)

var (
	checkDir           string
	checkRequired      string
	checkFilter        string
	checkSkip          string
	checkRecursive     bool
	checkStrictDynamic bool
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Audit and validate resource tags against governance policy rules",
	Long: `Enforces tagging policies and schema compliance across Terraform IaC resources.
Fails with exit code 1 if required tags are missing or fail regex pattern validation.

Example:
  hoptag check -d ./infra --required "Environment=^(dev|staging|prod)$,CostCenter=^CC-[0-9]{4}$,ManagedBy"`,
	RunE: runCheck,
}

func init() {
	checkCmd.Flags().StringVarP(&checkDir, "dir", "d", ".", "Target directory containing Terraform files")
	checkCmd.Flags().StringVar(&checkRequired, "required", "", "Comma-separated required tag rules (e.g. 'Environment=^(dev|prod)$,CostCenter')")
	checkCmd.Flags().StringVar(&checkFilter, "filter", "", "Regex to filter which resource types to check")
	checkCmd.Flags().StringVar(&checkSkip, "skip", "", "Regex to exclude resource types from checking")
	checkCmd.Flags().BoolVarP(&checkRecursive, "recursive", "r", true, "Recursively scan subdirectories for .tf files")
	checkCmd.Flags().BoolVar(&checkStrictDynamic, "strict-dynamic", false, "Fail if tag values use dynamic variables that cannot be evaluated statically")

	_ = checkCmd.MarkFlagRequired("required")
	rootCmd.AddCommand(checkCmd)
}

func runCheck(cmd *cobra.Command, args []string) error {
	printBanner()

	rulesInput := strings.TrimSpace(checkRequired)
	if rulesInput == "" {
		return fmt.Errorf("--required rules flag cannot be empty")
	}

	rules, err := models.ParseRules(rulesInput)
	if err != nil {
		return fmt.Errorf("failed to parse required rules: %w", err)
	}

	var filterRe *regexp.Regexp
	if checkFilter != "" {
		filterRe, err = regexp.Compile(checkFilter)
		if err != nil {
			return fmt.Errorf("invalid filter regex %q: %w", checkFilter, err)
		}
	}

	var skipRe *regexp.Regexp
	if checkSkip != "" {
		skipRe, err = regexp.Compile(checkSkip)
		if err != nil {
			return fmt.Errorf("invalid skip regex %q: %w", checkSkip, err)
		}
	}

	opts := models.CheckOptions{
		Dir:         checkDir,
		Rules:       rules,
		FilterRegex: filterRe,
		SkipRegex:   skipRe,
		Recursive:   checkRecursive,
	}

	if !quiet {
		fmt.Printf("%s %s\n", bold("Auditing Directory:"), cyan(checkDir))
		fmt.Println(bold("Required Governance Rules:"))
		for _, r := range rules {
			if r.Pattern != nil {
				fmt.Printf("   - %s: matches %s\n", bold(r.Key), yellow(r.Pattern.String()))
			} else {
				fmt.Printf("   - %s: %s\n", bold(r.Key), yellow("presence required"))
			}
		}
		fmt.Println()
	}

	filePaths, err := hcl.DiscoverFiles(checkDir, checkRecursive)
	if err != nil {
		return err
	}
	if len(filePaths) == 0 {
		if !quiet {
			fmt.Printf("No Terraform (.tf) files found in %q\n", checkDir)
		}
		return nil
	}

	registry := providers.DefaultRegistry()
	var allViolations []models.Violation
	totalResources := 0
	compliantResources := 0

	for _, path := range filePaths {
		fileAST, _, err := hcl.LoadFile(path)
		if err != nil {
			return fmt.Errorf("failed to parse %q: %w", path, err)
		}

		violations, resCount, compCount, err := validator.ValidateFile(fileAST, path, opts, registry)
		if err != nil {
			return fmt.Errorf("error checking %q: %w", path, err)
		}

		totalResources += resCount
		compliantResources += compCount

		for _, v := range violations {
			if v.Type == models.ViolationDynamicTag && !checkStrictDynamic {
				continue
			}
			allViolations = append(allViolations, v)
		}
	}

	if len(allViolations) > 0 {
		if !quiet {
			fmt.Println(red(bold("POLICY VIOLATIONS DETECTED:")))
			fmt.Println("--------------------------------------------------------------------------------")
			for i, v := range allViolations {
				relPath, _ := filepath.Rel(checkDir, v.FilePath)
				if relPath == "" {
					relPath = v.FilePath
				}
				fmt.Printf("[%d] %s: %s.%s\n", i+1, bold(relPath), cyan(v.ResourceType), cyan(v.ResourceName))
				fmt.Printf("    Reason: %s\n", red(v.Message))
				if v.CurrentValue != "" {
					fmt.Printf("    Actual:   %q\n", v.CurrentValue)
				}
				fmt.Printf("    Expected: %s\n", yellow(v.ExpectedRule))
				fmt.Println()
			}
			fmt.Println("--------------------------------------------------------------------------------")
			fmt.Printf("Summary: %d violation(s) found across %d resources in %d files.\n",
				len(allViolations), totalResources, len(filePaths))
		}
		// Return policy failure error
		return fmt.Errorf("tag policy check failed with %d violation(s)", len(allViolations))
	}

	if !quiet {
		fmt.Printf("%s All %d resource(s) across %d file(s) comply with tagging policies!\n",
			green(bold("PASSED:")), compliantResources, len(filePaths))
	}

	return nil
}
