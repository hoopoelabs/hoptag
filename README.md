# 🏷️ hoptag

[![Go Report Card](https://goreportcard.com/badge/github.com/hoopoelabs/hoptag)](https://goreportcard.com/report/github.com/hoopoelabs/hoptag)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/hoopoelabs/hoptag)](go.mod)

> **Modern, AST-Safe Infrastructure as Code (IaC) Auto-Tagger & Governance CLI**  
> Developed by **Hoopoe Labs**

`hoptag` automatically scans Terraform and OpenTofu infrastructure-as-code files and injects, merges, and validates resource tags and labels across **AWS**, **Microsoft Azure**, and **Google Cloud Platform (GCP)**.

Unlike legacy taggers that rely on crude string manipulation or destructive overwrites, `hoptag` uses HashiCorp's official `hclwrite` parser to surgically modify HCL syntax trees while keeping comments, line breaks, and formatting pristine.

---

## 🚀 Key Features

- **🛡️ AST-Safe & Non-Destructive**: Preserves developer comments, blank lines, and formatting using native HCL AST parsing.
- **☁️ Multi-Cloud Native**: Seamlessly maps standard tag sets to AWS `tags`, Azure `tags`, and GCP `labels`.
- **⚡ Non-Destructive CI Mode (`--mode override`)**: Generates Terraform-native `_hoptag_override.tf` files instead of dirtying your git working tree.
- **🔄 Smart Attribute Merging**: Preserves custom existing tags and intelligently wraps dynamic expressions (e.g. `var.common_tags`) with `merge()`.
- **📋 Governance & Policy Enforcement (`hoptag check`)**: Validates existing tags against mandatory keys and regex schema rules before `terraform plan` or `terraform apply`.
- **🎯 Precise Granularity**: Filter or skip resources using regex (`--filter`, `--skip`).

---

## 📦 Installation

### From Source (Go 1.22+)
```bash
go install github.com/hoopoelabs/hoptag@latest
```

### Build Locally
```bash
git clone https://github.com/hoopoelabs/hoptag.git
cd hoptag
make build
# Binary is in ./bin/hoptag
```

---

## 🛠️ Usage & Commands

### 1. `hoptag apply` — Inject & Merge Tags

Automatically injects tags into all compatible resources in the target directory.

#### Reading Tags Directly From a File (Any Filename or Extension!)
Pass the file path directly as an argument or via `-f / --tags-file`. `hoptag` automatically detects YAML, JSON, or key=value formats:
```bash
# Direct argument (no flags needed!)
hoptag apply -d ./infra my-tags-file

# Via -f or --tags-file flag
hoptag apply -d ./infra -f ./config/tags.yaml

# Via -t flag
hoptag apply -d ./infra -t ./tags.env
```
*Supported file contents: YAML (`key: value`), JSON (`{"key":"value"}`), or line-by-line (`key=value` or `key: value`).*

#### Inline JSON Input Format
```bash
hoptag apply -d ./infra -t '{"Environment":"production","CostCenter":"FIN-104","ManagedBy":"hoptag"}'
```

#### Inline Key-Value Shorthand Format
```bash
hoptag apply -d ./infra -t "Environment=production,CostCenter=FIN-104,Team=DevOps"
```

#### Environment Variable Fallback
```bash
export HOPTAG_TAGS='{"Environment":"production","CostCenter":"FIN-104"}'
hoptag apply -d ./infra
```

#### Non-Destructive CI Mode (Terraform Native Override)
Generates `_hoptag_override.tf` in the target directory without mutating any source files:
```bash
hoptag apply -d ./infra -t "Environment=staging" --mode=override
```

#### Dry Run Mode
Preview what would be tagged without writing changes to disk:
```bash
hoptag apply -d ./infra -t "Environment=production" --dry-run
```

#### Filter or Skip Resource Types
```bash
# Only tag AWS resources
hoptag apply -d ./infra -t "Environment=prod" --filter "^aws_.*"

# Skip databases
hoptag apply -d ./infra -t "Environment=prod" --skip ".*_db_instance$"
```

#### Flags for `apply`
| Flag | Shorthand | Description | Default |
|------|-----------|-------------|---------|
| `--dir` | `-d` | Path to directory containing `.tf` files | `.` |
| `--tags` | `-t` | Inline tags string or file path | `$HOPTAG_TAGS` |
| `--tags-file` | `-f` | Path to file containing tags (YAML, JSON, .env) | `""` |
| `--mode` | `-m` | Tagging mode: `in-place`, `override`, `rename` | `in-place` |
| `--filter` | | Regex pattern to match resource types to tag | `""` |
| `--skip` | | Regex pattern to exclude resource types | `""` |
| `--preserve-existing` | | Keep existing tag values instead of overriding | `false` |
| `--recursive` | `-r` | Recursively scan subdirectories for `.tf` files | `true` |
| `--dry-run` | | Preview tagging changes without writing to disk | `false` |

---

### 2. `hoptag check` — Governance & Policy Linting

Fails with **exit code 1** if required tags are missing or fail regex patterns. Ideal for CI pre-commit hooks and pipeline gates before running `terraform plan`.

```bash
hoptag check -d ./infra --required "Environment=^(dev|staging|production)$,CostCenter=^CC-[0-9]{4}$,ManagedBy"
```

#### Rules Syntax
- `Key`: Requires the tag key to exist.
- `Key=^Regex$`: Requires the tag key to exist AND its value to match the regex.
- Multiple rules separated by commas.

Example CI check failure output:
```text
❌ GOVERNANCE VIOLATIONS DETECTED:
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

## 🧩 Tagging Logic & Provider Mapping

`hoptag` automatically detects the provider for each resource block:

| Provider | Resource Prefix | Injected Attribute | Example Resource |
|----------|-----------------|--------------------|------------------|
| **AWS** | `aws_` | `tags = { ... }` | `aws_instance`, `aws_s3_bucket`, `aws_vpc` |
| **Azure** | `azurerm_` | `tags = { ... }` | `azurerm_resource_group`, `azurerm_linux_virtual_machine` |
| **GCP** | `google_` | `labels = { ... }` | `google_compute_instance`, `google_storage_bucket` |

### How Existing Attributes Are Merged

1. **Untagged Resource**:
   Creates a new map literal formatted with standard Terraform indentation.
2. **Existing Map Literal**:
   ```hcl
   # Original
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
   *Comments and unconflicted custom keys are 100% preserved.*
3. **Dynamic Reference / Variable**:
   ```hcl
   # Original
   tags = var.common_tags

   # After: hoptag apply -t "Environment=prod"
   tags = merge(var.common_tags, {
     Environment = "prod"
   })
   ```

---

## 🔄 CI/CD Workflows

### GitHub Actions (Non-Destructive Override Mode)
```yaml
name: Infrastructure Pipeline

on: [pull_request]

jobs:
  tag-and-plan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Install hoptag
        run: go install github.com/hoopoelabs/hoptag@latest

      - name: Verify Tag Compliance
        run: |
          hoptag check -d ./terraform \
            --required "Environment=^(staging|production)$,CostCenter=^CC-[0-9]{4}$"

      - name: Auto-tag Resources (Non-destructive)
        run: |
          hoptag apply -d ./terraform \
            -t "Environment=${{ github.ref_name }},ManagedBy=GitHubActions" \
            --mode=override

      - name: Terraform Plan
        run: |
          terraform init
          terraform plan
```

---

## 🏗️ Architecture

```
hoptag/
├── cmd/
│   ├── root.go             # Root Cobra command & styling
│   ├── apply.go            # 'hoptag apply' implementation
│   ├── check.go            # 'hoptag check' policy evaluation
│   └── version.go          # Version info
├── internal/
│   ├── hcl/
│   │   ├── parser.go       # File discovery & AST loading via hclwrite
│   │   ├── injector.go     # Surgical AST tag injection & token merging
│   │   └── writer.go       # In-place, override, and rename writers
│   ├── providers/
│   │   ├── provider.go     # Provider interface & Registry
│   │   ├── aws.go          # AWS tagging rules (tags)
│   │   ├── azure.go        # Azure tagging rules (tags)
│   │   └── gcp.go          # GCP labeling rules (labels)
│   ├── validator/
│   │   └── validator.go    # Governance & regex policy engine
│   └── models/
│       └── tag.go          # Tag data structures & CLI options
├── test/
│   ├── fixtures/           # Fixture .tf files
│   └── integration_test.go # End-to-end integration tests
├── Makefile
├── go.mod
└── README.md
```

---

## 🤝 Contributing

Contributions are welcome!
1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Run tests: `make test`
5. Push to the branch (`git push origin feature/amazing-feature`)
6. Open a Pull Request

---

## 📄 License

Licensed under the Apache License, Version 2.0. Developed with ❤️ by **Hoopoe Labs**.
