# hoptag

[![Go Report Card](https://goreportcard.com/badge/github.com/hoopoelabs/hoptag)](https://goreportcard.com/report/github.com/hoopoelabs/hoptag)
[![License: Hoopoe Labs Free-Use](https://img.shields.io/badge/License-Hoopoe%20Labs%20Free--Use-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/hoopoelabs/hoptag)](go.mod)

`hoptag` is a CLI tool developed by Hoopoe Labs for scanning Terraform and OpenTofu infrastructure-as-code files and automatically injecting, merging, and validating resource tags and labels across AWS, Microsoft Azure, and Google Cloud Platform (GCP).

Unlike string-replacement or regex-based tools, `hoptag` operates on the HCL syntax tree via HashiCorp's `hclwrite` library, ensuring that comments, line breaks, indentation, and existing expressions remain intact.

---

## Features

- **AST-Safe Modification**: Uses HashiCorp's official `hclwrite` parser to modify configuration blocks without destroying comments, blank lines, or existing formatting.
- **Multi-Cloud Mapping**: Maps tags to the appropriate provider attributes: AWS (`tags`), Azure (`tags`), and GCP (`labels`).
- **Non-Destructive Override Mode (`--mode override`)**: Generates Terraform-native `_hoptag_override.tf` files instead of mutating source files, preventing dirty git states during CI/CD execution.
- **Expression Merging**: Merges new keys into existing map literals and wraps dynamic expressions (such as `tags = var.common_tags`) using Terraform's native `merge()` function.
- **Tag Policy Validation (`hoptag check`)**: Validates tag presence and validates values against regex patterns before plan or apply steps run.
- **Granular Targeting**: Includes `--filter` and `--skip` flags to match or exclude specific resource types via regular expressions.
- **Format-Agnostic File Inputs**: Accepts tag definitions directly from files (YAML, JSON, or key=value formats) without requiring specific extensions or syntax wrappers.

---

## Installation

### From Source (Go 1.22+)

```bash
go install github.com/hoopoelabs/hoptag@latest
```

### Build from Repository

```bash
git clone https://github.com/hoopoelabs/hoptag.git
cd hoptag
make build
# Binary is placed in ./bin/hoptag
```

### Pre-built Binaries

Pre-compiled binaries for Linux (amd64, arm64), macOS (Intel, Apple Silicon), and Windows are available on the [Releases](https://github.com/hoopoelabs/hoptag/releases) page.

---

## Usage

### 1. `hoptag apply`

Injects and merges tags across all supported resources in the target directory.

#### Passing a File Directly

You can pass a tag configuration file directly as an argument, or use `-f / --tags-file`. Content format (YAML, JSON, or line-delimited key=value) is detected automatically:

```bash
# Direct positional argument
hoptag apply -d ./infra tags.yaml

# Using the -f flag
hoptag apply -d ./infra -f ./config/tags.json

# Using key=value properties file
hoptag apply -d ./infra -f ./tags.env
```

#### Inline Tags

Tags can also be provided inline as JSON or key-value pairs:

```bash
# JSON format
hoptag apply -d ./infra -t '{"Environment":"production","CostCenter":"FIN-104"}'

# Key-value shorthand
hoptag apply -d ./infra -t "Environment=production,CostCenter=FIN-104"
```

#### Environment Variables

If `--tags` or `--tags-file` is not provided, `hoptag` falls back to the `HOPTAG_TAGS` environment variable:

```bash
export HOPTAG_TAGS='{"Environment":"production","CostCenter":"FIN-104"}'
hoptag apply -d ./infra
```

#### Non-Destructive CI Mode

Generates `_hoptag_override.tf` files containing only the tag definitions. Terraform natively merges override files without modifying your original files:

```bash
hoptag apply -d ./infra -f tags.yaml --mode=override
```

#### Dry-Run Mode

Displays what would be tagged without modifying files on disk:

```bash
hoptag apply -d ./infra -f tags.yaml --dry-run
```

#### Filtering Resources

```bash
# Only tag AWS resources
hoptag apply -d ./infra -t "Env=prod" --filter "^aws_.*"

# Exclude specific resource types
hoptag apply -d ./infra -t "Env=prod" --skip ".*_db_instance$"
```

#### Command Options (`apply`)

| Flag | Short | Description | Default |
|---|---|---|---|
| `--dir` | `-d` | Path to directory containing `.tf` files | `.` |
| `--tags` | `-t` | Inline tag string (JSON or key=value) or file path | `$HOPTAG_TAGS` |
| `--tags-file` | `-f` | Path to tag file (YAML, JSON, or key=value) | `""` |
| `--mode` | `-m` | Application mode: `in-place`, `override`, or `rename` | `in-place` |
| `--preserve-existing` | | Preserve existing tag values on conflict | `false` |
| `--filter` | | Regex pattern matching resource types to tag | `""` |
| `--skip` | | Regex pattern matching resource types to exclude | `""` |
| `--recursive` | `-r` | Recursively scan subdirectories for `.tf` files | `true` |
| `--dry-run` | | Preview changes without modifying files | `false` |

---

### 2. `hoptag check`

Audits resources and exits with code 1 if required tags are missing or fail regex validation. Useful in CI pipelines to gate `terraform plan`.

```bash
hoptag check -d ./infra --required "Environment=^(dev|staging|production)$,CostCenter=^CC-[0-9]{4}$,ManagedBy"
```

#### Rule Definitions

- `Key`: Tag key must be present on the resource.
- `Key=^Regex$`: Tag key must be present and match the regex pattern.
- Comma-separated for multiple rules.

Sample failure output:

```text
GOVERNANCE VIOLATIONS DETECTED:
--------------------------------------------------------------------------------
[1] infra/compute.tf: aws_instance.web
    Reason: Missing required tag "CostCenter"
    Expected: CostCenter=^CC-[0-9]{4}$

[2] infra/storage.tf: google_storage_bucket.assets
    Reason: Tag "Environment" value "testing" does not match pattern "^(dev|staging|production)$"
    Actual:   "testing"
    Expected: ^(dev|staging|production)$
--------------------------------------------------------------------------------
Summary: 2 violation(s) found across 8 resources in 3 files.
```

---

## Provider Support

`hoptag` automatically detects provider types by resource prefix:

| Provider | Resource Prefix | Target Attribute | Handled Exceptions |
|---|---|---|---|
| AWS | `aws_` | `tags` | Skips non-taggable association blocks (`aws_route`, `aws_security_group_rule`, etc.) |
| Azure | `azurerm_` | `tags` | Skips untaggable sub-resources (`azurerm_role_assignment`, associations) |
| GCP | `google_` | `labels` | Skips IAM policy bindings, project services, and non-labelable resources |

### Merging Semantics

1. **Untagged Resource**: Injects a formatted map block.
2. **Existing Map**:
   ```hcl
   # Before
   tags = {
     Name        = "app-server"
     Environment = "dev"
   }

   # After: hoptag apply -t "Environment=prod,CostCenter=FIN-104"
   tags = {
     Name        = "app-server"
     Environment = "prod"
     CostCenter  = "FIN-104"
   }
   ```
   Comments and unconflicted keys are preserved.
3. **Dynamic Expression**:
   ```hcl
   # Before
   tags = var.common_tags

   # After
   tags = merge(var.common_tags, {
     Environment = "prod"
   })
   ```

---

## CI/CD Pipeline Integration

### GitHub Actions Example

```yaml
name: Terraform Pipeline

on: [pull_request]

jobs:
  validate-and-tag:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Install hoptag
        run: go install github.com/hoopoelabs/hoptag@latest

      - name: Enforce Tag Compliance
        run: |
          hoptag check -d ./terraform \
            --required "Environment=^(staging|production)$,CostCenter=^CC-[0-9]{4}$"

      - name: Inject CI Tags (Non-destructive)
        run: |
          hoptag apply -d ./terraform \
            -t "Environment=${{ github.ref_name }},ManagedBy=CI" \
            --mode=override

      - name: Terraform Plan
        run: |
          terraform init
          terraform plan
```

---

## Project Structure

```
hoptag/
├── cmd/
│   ├── root.go             # Root command definition
│   ├── apply.go            # 'hoptag apply' implementation
│   ├── check.go            # 'hoptag check' policy validation
│   └── version.go          # Version info
├── internal/
│   ├── hcl/
│   │   ├── parser.go       # File discovery and AST loading
│   │   ├── injector.go     # AST tag injection and token merging
│   │   └── writer.go       # In-place, override, and rename writers
│   ├── providers/
│   │   ├── provider.go     # Provider interface and registry
│   │   ├── aws.go          # AWS tagging rules
│   │   ├── azure.go        # Azure tagging rules
│   │   └── gcp.go          # GCP labeling rules
│   ├── validator/
│   │   └── validator.go    # Governance evaluation engine
│   └── models/
│       └── tag.go          # Tag data structures and parsers
├── test/
│   ├── fixtures/           # Terraform fixture files
│   └── integration_test.go # End-to-end integration tests
├── Makefile
├── go.mod
└── README.md
```

---

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/improvement`)
3. Commit your changes (`git commit -m 'feat: description'`)
4. Verify tests pass: `make test`
5. Push to the branch (`git push origin feature/improvement`)
6. Open a Pull Request

---

## License

Licensed under the **Hoopoe Labs Source-Available License** (Free for personal, internal, and commercial use with mandatory attribution and anti-rebranding terms). See [LICENSE](LICENSE) and [NOTICE](NOTICE) for full terms.
