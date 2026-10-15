package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hoopoelabs/hoptag/internal/hcl"
	"github.com/hoopoelabs/hoptag/internal/models"
	"github.com/hoopoelabs/hoptag/internal/providers"
	"github.com/spf13/cobra"
)

var (
	applyDir              string
	applyTags             string
	applyTagsFile         string
	applyMode             string
	applyFilter           string
	applySkip             string
	applyPreserveExisting bool
	applyRecursive        bool
	applyDryRun           bool
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Inject and merge tags/labels across Terraform IaC resources",
	Long: `Applies specified tags/labels to all compatible AWS, Azure, and GCP resources.

Supports reading directly from a file:
  hoptag apply -d ./infra -f tags.yaml
  hoptag apply -d ./infra --tags-file tags.json

Supports JSON format:
  hoptag apply -d ./infra -t '{"Environment":"production","CostCenter":"FIN-104","ManagedBy":"HoopoeLabs"}'

Supports key-value shorthand format:
  hoptag apply -d ./infra -t "Environment=production,CostCenter=FIN-104"

Supports non-destructive override mode:
  hoptag apply -d ./infra -f tags.yaml --mode=override`,
	RunE: runApply,
}

func init() {
	applyCmd.Flags().StringVarP(&applyDir, "dir", "d", ".", "Target directory containing Terraform files")
	applyCmd.Flags().StringVarP(&applyTags, "tags", "t", "", "Tags to apply in JSON or Key=Val format (or HOPTAG_TAGS env)")
	applyCmd.Flags().StringVarP(&applyTagsFile, "tags-file", "f", "", "Path to file containing tags (JSON, YAML, or .env format)")
	applyCmd.Flags().StringVarP(&applyMode, "mode", "m", models.ModeInPlace, "Tagging mode: 'in-place', 'override', or 'rename'")
	applyCmd.Flags().StringVar(&applyFilter, "filter", "", "Regex to filter which resource types to tag (e.g. '^aws_.*')")
	applyCmd.Flags().StringVar(&applySkip, "skip", "", "Regex to exclude resource types from tagging")
	applyCmd.Flags().BoolVar(&applyPreserveExisting, "preserve-existing", false, "Preserve existing tag values over injected tags on conflict")
	applyCmd.Flags().BoolVarP(&applyRecursive, "recursive", "r", true, "Recursively scan subdirectories for .tf files")
	applyCmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "Preview tagging changes without writing to disk")

	rootCmd.AddCommand(applyCmd)
}

func runApply(cmd *cobra.Command, args []string) error {
	printBanner()

	// Support passing tags or tag file directly as a CLI argument:
	// Example: hoptag apply my-tags  OR  hoptag apply -d ./infra my-tags
	if applyTagsFile == "" && applyTags == "" && len(args) > 0 {
		candidate := strings.TrimSpace(args[0])
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			applyTagsFile = candidate
		} else {
			applyTags = candidate
		}
	}

	// 1. Resolve tags from file, CLI flag, or environment variable
	tags := make(models.TagMap)

	if applyTagsFile != "" {
		fileTags, err := models.ParseTagsFromFile(applyTagsFile)
		if err != nil {
			return err
		}
		for k, v := range fileTags {
			tags[k] = v
		}
	}

	tagInput := strings.TrimSpace(applyTags)
	if tagInput == "" && len(tags) == 0 {
		tagInput = strings.TrimSpace(os.Getenv("HOPTAG_TAGS"))
	}
	if tagInput == "" && len(tags) == 0 {
	}

	if tagInput != "" {
		cliTags, err := models.ParseTags(tagInput)
		if err != nil {
			return fmt.Errorf("failed to parse tags: %w", err)
		}
		for k, v := range cliTags {
			tags[k] = v
		}
	}

	if len(tags) == 0 {
		return fmt.Errorf("tags must be specified via --tags/-t, --tags-file/-f, or HOPTAG_TAGS environment variable")
	}

	// 2. Validate mode
	normalizedMode := strings.ToLower(strings.TrimSpace(applyMode))
		normalizedMode = models.ModeRename
	}
	if normalizedMode != models.ModeInPlace && normalizedMode != models.ModeOverride && normalizedMode != models.ModeRename {
		return fmt.Errorf("invalid mode %q: must be 'in-place', 'override', or 'rename'", applyMode)
	}

	// 3. Compile filter & skip regexes
	var filterRe *regexp.Regexp
	if applyFilter != "" {
		var err error
		filterRe, err = regexp.Compile(applyFilter)
		if err != nil {
			return fmt.Errorf("invalid filter regex %q: %w", applyFilter, err)
		}
	}

	var skipRe *regexp.Regexp
	if applySkip != "" {
		var err error
		skipRe, err = regexp.Compile(applySkip)
		if err != nil {
			return fmt.Errorf("invalid skip regex %q: %w", applySkip, err)
		}
	}

	opts := models.ApplyOptions{
		Dir:              applyDir,
		Tags:             tags,
		Mode:             normalizedMode,
		FilterRegex:      filterRe,
		SkipRegex:        skipRe,
		PreserveExisting: applyPreserveExisting,
		Recursive:        applyRecursive,
	}

	if !quiet {
		fmt.Printf("%s %s (mode: %s, preserve-existing: %t)\n",
			bold("Target Directory:"), cyan(applyDir), yellow(normalizedMode), applyPreserveExisting)
		fmt.Println(bold("Tags to Inject:"))
		for k, v := range tags {
			fmt.Printf("   - %s: %s\n", bold(k), v)
		}
		if applyDryRun {
			fmt.Printf("%s\n", yellow("[DRY RUN] No files will be modified on disk."))
		}
		fmt.Println()
	}

	// 4. Discover files
	filePaths, err := hcl.DiscoverFiles(applyDir, applyRecursive)
	if err != nil {
		return err
	}
	if len(filePaths) == 0 {
		if !quiet {
			fmt.Printf("No Terraform (.tf) files found in %q\n", applyDir)
		}
		return nil
	}

	registry := providers.DefaultRegistry()
	writer := hcl.NewWriter(normalizedMode)

	// 5. Handle Override Mode
	if normalizedMode == models.ModeOverride {
		return handleOverrideMode(filePaths, opts, registry)
	}

	// 6. Handle In-Place or Rename Mode
	totalFilesModified := 0
	totalResourcesTagged := 0

	for _, path := range filePaths {
		fileAST, _, err := hcl.LoadFile(path)
		if err != nil {
			return fmt.Errorf("failed to parse %q: %w", path, err)
		}

		res, err := hcl.InjectTags(fileAST, path, opts, registry)
		if err != nil {
			return fmt.Errorf("failed to inject tags into %q: %w", path, err)
		}

		if res.Modified {
			totalFilesModified++
			totalResourcesTagged += res.ResourcesTagged

			if !quiet {
				relPath, _ := filepath.Rel(applyDir, path)
				if relPath == "" {
					relPath = path
				}
				fmt.Printf("  %s %s (%d resources tagged)\n",
					green("[OK]"), bold(relPath), res.ResourcesTagged)
				if verbose {
					for _, stat := range res.Stats {
						fmt.Printf("      -> [%s] %s.%s (%d injected, %d updated)\n",
							stat.AttributeName, stat.ResourceType, stat.ResourceName, stat.InjectedTags, stat.UpdatedTags)
					}
				}
			}

			if !applyDryRun {
				if err := writer.WriteFile(path, fileAST); err != nil {
					return fmt.Errorf("failed to save %q: %w", path, err)
				}
			}
		} else if verbose && !quiet {
			relPath, _ := filepath.Rel(applyDir, path)
			fmt.Printf("  - %s (no taggable resources)\n", relPath)
		}
	}

	// Summary
	if !quiet {
		fmt.Println()
		fmt.Printf("%s Total files scanned: %d | Files updated: %d | Resources tagged: %d\n",
			green(bold("SUCCESS:")), len(filePaths), totalFilesModified, totalResourcesTagged)
	}

	return nil
}

func handleOverrideMode(filePaths []string, opts models.ApplyOptions, registry *providers.Registry) error {
	// Group files by directory
	dirFiles := make(map[string][]*hclwrite.File)
	for _, path := range filePaths {
		dir := filepath.Dir(path)
		fileAST, _, err := hcl.LoadFile(path)
		if err != nil {
			return fmt.Errorf("failed to parse %q: %w", path, err)
		}
		dirFiles[dir] = append(dirFiles[dir], fileAST)
	}

	totalOverrides := 0
	totalResourcesTagged := 0

	for dir, astList := range dirFiles {
		outPath, count, err := hcl.WriteOverrideFile(dir, astList, opts, registry)
		if err != nil {
			return err
		}
		if count > 0 {
			totalOverrides++
			totalResourcesTagged += count
			if !quiet {
				relPath, _ := filepath.Rel(opts.Dir, outPath)
				if relPath == "" {
					relPath = outPath
				}
				fmt.Printf("  %s %s (%d resources overridden)\n",
					green("[OK]"), bold(relPath), count)
			}
		}
	}

	if !quiet {
		fmt.Println()
		fmt.Printf("%s Generated %d override file(s) across %d tagged resources.\n",
			green(bold("SUCCESS:")), totalOverrides, totalResourcesTagged)
		fmt.Println("   Existing .tf files were left completely untouched.")
	}

	return nil
}
