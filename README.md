# hoptag

[![License: Hoopoe Labs Free-Use](https://img.shields.io/badge/License-Hoopoe%20Labs%20Free--Use-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8.svg)](go.mod)

`hoptag` is a CLI tool developed by Hoopoe Labs for scanning Terraform and OpenTofu infrastructure-as-code files and automatically injecting, merging, and validating resource tags and labels across AWS, Microsoft Azure, and Google Cloud Platform (GCP).

Unlike string-replacement or regex-based tools, `hoptag` operates directly on the HCL syntax tree via HashiCorp's `hclwrite` library, ensuring that comments, line breaks, indentation, and existing expressions remain intact.

---

## Features

- **AST-Safe Parsing**: Uses HashiCorp's official `hclwrite` parser to modify configuration blocks without destroying comments, blank lines, or existing formatting.
- **Multi-Cloud Mapping**: Maps standard tags to the appropriate provider attributes: AWS (`tags`), Azure (`tags`), and GCP (`labels`).
- **Non-Destructive Override Mode (`--mode override`)**: Generates Terraform-native `_hoptag_override.tf` files instead of mutating source files, preventing dirty git states during CI/CD execution.
- **Expression Merging**: Merges new keys into existing map literals and wraps dynamic expressions (such as `tags = var.common_tags`) using Terraform's native `merge()` function.
- **Tag Policy Validation (`hoptag check`)**: Validates tag presence and validates values against regex patterns before plan or apply steps run.
- **Granular Targeting**: Includes `--filter` and `--skip` flags to match or exclude specific resource types via regular expressions.
- **Format-Agnostic File Inputs**: Accepts tag definitions directly from files (YAML, JSON, or key=value formats) without requiring specific extensions or syntax wrappers.
- **Pluggable Architecture**: Easily extended to support additional cloud providers, custom resources, or specialized tagging schemas.

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

```bash
hoptag apply [flags] [file-or-tags]
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

## Tag File Input Capabilities

`hoptag` provides flexible, extension-agnostic file loading. You are never forced to use specific file extensions (such as `.json` or `.yaml`) or complex syntax wrappers.

### Three Ways to Supply Tag Files

1. **Direct Positional Argument** (no flags needed):
   ```bash
   hoptag apply -d ./infra my-tags-file
   ```
2. **Via the `-f / --tags-file` Flag**:
   ```bash
   hoptag apply -d ./infra -f ./configs/production-tags
   ```
3. **Via the `-t / --tags` Flag**:
   ```bash
   # Automatically detects if the argument is a file on disk
   hoptag apply -d ./infra -t ./configs/production-tags
   ```

### Supported Content Formats

`hoptag` automatically inspects the content structure of any file regardless of its filename or extension:

#### 1. YAML Format
```yaml
Environment: production
CostCenter: FIN-104
ManagedBy: hoptag
Project: Phoenix
```

#### 2. JSON Format
```json
{
  "Environment": "production",
  "CostCenter": "FIN-104",
  "ManagedBy": "hoptag"
}
```

#### 3. Line-Delimited Key-Value / Properties Format (`.env`, `.conf`, `.txt`)
```ini
# Lines beginning with # or // are ignored
Environment=production
CostCenter: FIN-104
ManagedBy=hoptag
```

### Precedence and Overriding

When combining a base tag file with inline CLI tags, inline CLI tags take precedence:
```bash
# Loads common tags from file, but overrides Environment to 'staging'
hoptag apply -d ./infra -f common-tags.yaml -t "Environment=staging"
```

---

## Non-Destructive CI Mode (`--mode override`)

In CI/CD environments, mutating source `.tf` files directly can create unwanted git diffs. `hoptag` supports a native override mode:

```bash
hoptag apply -d ./infra -f tags.yaml --mode=override
```

In this mode, `hoptag` does not touch your source `.tf` files. Instead, it generates a Terraform-native `_hoptag_override.tf` file containing partial resource definitions:

```hcl
// Generated automatically by hoptag (Hoopoe Labs). DO NOT EDIT MANUALLY.

resource "aws_instance" "web" {
  tags = {
    CostCenter  = "FIN-104"
    Environment = "production"
    ManagedBy   = "hoptag"
  }
}
```

Terraform natively merges `*_override.tf` files with your existing resources during `terraform plan` and `terraform apply`.

---

## 2. `hoptag check` (Governance and Policy Linting)

Validates that resources comply with tag requirements and schema regex patterns. Fails with exit code 1 if required tags are missing or invalid, making it ideal as a CI gate before `terraform plan`.

```bash
hoptag check -d ./infra --required "Environment=^(dev|staging|production)$,CostCenter=^CC-[0-9]{4}$,ManagedBy"
```

### Rule Syntax

- `Key`: Tag key must be present on the resource.
- `Key=^Regex$`: Tag key must be present and match the regex pattern.
- Rules are comma-separated.

### Sample Output on Policy Failure

```text
POLICY VIOLATIONS DETECTED:
--------------------------------------------------------------------------------
[1] compute.tf: aws_instance.web
    Reason: Missing required tag "CostCenter"
    Expected: CostCenter=^CC-[0-9]{4}$

[2] storage.tf: google_storage_bucket.assets
    Reason: Tag "Environment" value "qa" does not match pattern "^(dev|staging|production)$"
    Actual:   "qa"
    Expected: ^(dev|staging|production)$
--------------------------------------------------------------------------------
Summary: 2 violation(s) found across 8 resources in 3 files.
```

---

## Provider Support and Merging Semantics

`hoptag` automatically detects the provider by inspecting resource prefixes:

| Provider | Resource Prefix | Target Attribute | Handled Exceptions |
|---|---|---|---|
| **AWS** | `aws_` | `tags` | Skips non-taggable association blocks (`aws_route`, `aws_security_group_rule`, etc.) |
| **Azure** | `azurerm_` | `tags` | Skips untaggable sub-resources (`azurerm_role_assignment`, associations) |
| **GCP** | `google_` | `labels` | Skips IAM policy bindings, project services, and non-labelable resources |

### Merging Rules

1. **Untagged Resource**: Injects a formatted map attribute with standardized indentation.
2. **Existing Map Literal**:
   ```hcl
   # Before
   tags = {
     # Important custom tag
     BackupPlan  = "Daily"
     Environment = "dev"
   }

   # After: hoptag apply -t "Environment=prod,CostCenter=FIN-104"
   tags = {
     # Important custom tag
     BackupPlan  = "Daily"
     Environment = "prod"
     CostCenter  = "FIN-104"
   }
   ```
   Comments and unconflicted keys are preserved.
3. **Dynamic Expression**:
   ```hcl
   # Before
   tags = var.common_tags

   # After: hoptag apply -t "Environment=prod"
   tags = merge(var.common_tags, {
     Environment = "prod"
   })
   ```

---

## Extensibility and Custom Providers

`hoptag` is architected with a decoupled provider registry that allows developers to add support for new clouds or specialized resource rules.

### The `Provider` Interface

Every cloud provider in `hoptag` implements the `Provider` interface defined in `internal/providers/provider.go`:

```go
type Provider interface {
    // Name returns the friendly display name of the provider (e.g. "AWS", "Alibaba").
    Name() string

    // Prefix returns the Terraform resource type prefix (e.g. "aws_", "alicloud_").
    Prefix() string

    // CanTag reports whether the specified resource type supports tags or labels.
    CanTag(resourceType string) bool

    // TagAttributeName returns the attribute key used for tags (e.g. "tags" or "labels").
    TagAttributeName(resourceType string) string
}
```

### Implementing a Custom Provider

To add support for a new cloud provider (e.g., Alibaba Cloud or Kubernetes), create an implementation of the interface:

```go
package custom

import (
    "strings"
    "github.com/hoopoelabs/hoptag/internal/providers"
)

type AlibabaProvider struct {
    untaggable map[string]struct{}
}

func NewAlibabaProvider() *AlibabaProvider {
    return &AlibabaProvider{
        untaggable: map[string]struct{}{
            "alicloud_route_entry": {},
        },
    }
}

func (p *AlibabaProvider) Name() string                     { return "Alibaba" }
func (p *AlibabaProvider) Prefix() string                   { return "alicloud_" }
func (p *AlibabaProvider) TagAttributeName(r string) string { return "tags" }
func (p *AlibabaProvider) CanTag(resourceType string) bool {
    if !strings.HasPrefix(resourceType, p.Prefix()) {
        return false
    }
    _, blocked := p.untaggable[resourceType]
    return !blocked
}
```

### Registering the Provider

Register your provider with the registry:

```go
registry := providers.DefaultRegistry()
registry.Register(NewAlibabaProvider())
```

Once registered, `hoptag apply` and `hoptag check` will automatically recognize `alicloud_*` resources and tag or validate them according to your defined rules.

---

## Project Structure

```
hoptag/
├── cmd/
│   ├── root.go             # Root command definition, global flags, and banner styling
│   ├── apply.go            # 'hoptag apply' implementation (file resolution and tagging)
│   ├── check.go            # 'hoptag check' implementation (governance and regex policy engine)
│   └── version.go          # Version info and build metadata
├── internal/
│   ├── hcl/
│   │   ├── parser.go       # HCL file discovery, path normalization, and AST loading
│   │   ├── injector.go     # Surgical AST token manipulation and comment-preserving merger
│   │   ├── writer.go       # Writers for in-place updates, override files, and backups
│   │   ├── injector_test.go# Unit tests for tag injection, comment preservation, and filters
│   │   └── writer_test.go  # Unit tests for safe in-place, rename, and override file creation
│   ├── providers/
│   │   ├── provider.go     # Provider interface and concurrent Registry implementation
│   │   ├── aws.go          # AWS tagging rules (tags attribute and untaggable exclusion list)
│   │   ├── azure.go        # Azure tagging rules (tags attribute and untaggable exclusion list)
│   │   ├── gcp.go          # GCP labeling rules (labels attribute and untaggable exclusion list)
│   │   └── providers_test.go# Unit tests verifying provider detection and capability checks
│   ├── validator/
│   │   ├── validator.go    # Tag policy evaluator (presence checks and regex pattern matching)
│   │   └── validator_test.go# Unit tests for policy verification and violation detection
│   └── models/
│       ├── tag.go          # Data structures (TagMap, TagRule, ApplyOptions, CheckOptions)
│       └── tag_test.go     # Tests for JSON, YAML, and line-delimited file parsing
├── test/
│   ├── fixtures/           # Sample Terraform files (AWS, Azure, GCP, comments, dynamic tags)
│   └── integration_test.go # End-to-end integration tests (apply -> check -> verify)
├── .github/
│   └── workflows/
│       ├── ci.yml          # Continuous integration workflow (Ubuntu, macOS, Windows)
│       └── release.yml     # Automated multi-platform release workflow
├── .goreleaser.yaml        # GoReleaser configuration for cross-platform binary releases
├── Makefile                # Build, test, cross-compile (build-all), clean, and format targets
├── LICENSE                 # Hoopoe Labs Source-Available License (Attribution & Anti-Rebranding)
├── NOTICE                  # Permanent author credit and trademark notice
├── go.mod                  # Go module definition
└── README.md               # Project documentation
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
