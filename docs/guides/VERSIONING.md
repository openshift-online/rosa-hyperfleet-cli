# Versioning Guide

This project uses **Semantic Versioning** (SemVer) with automated version management via `go-semver-release`.

## Quick Start

```bash
# Check what the next version would be
make release-dry-run

# Create a new release
make release

# Push the release tag to GitHub
git push origin v0.2.0
```

## How It Works

### Conventional Commits

Versioning is based on [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Version Bump Rules

The commit message **type** determines the version bump:

| Commit Type                                 | Version Bump              | Example                            |
| ------------------------------------------- | ------------------------- | ---------------------------------- |
| `feat:`                                     | **MINOR** (0.1.0 → 0.2.0) | `feat: add JWT signing command`    |
| `fix:`                                      | **PATCH** (0.1.0 → 0.1.1) | `fix: S3 timeout in e2e tests`     |
| `perf:`                                     | **PATCH**                 | `perf: optimize Lambda packaging`  |
| `refactor:`                                 | **PATCH**                 | `refactor: simplify OIDC creation` |
| `BREAKING CHANGE:`                          | **MAJOR** (0.1.0 → 1.0.0) | See below                          |
| `docs:`, `chore:`, `test:`, `build:`, `ci:` | **PATCH**                 | Documentation, tooling changes     |

### Breaking Changes

To trigger a MAJOR version bump, include `BREAKING CHANGE:` in the commit footer:

```bash
git commit -m "feat: redesign Lambda API" -m "BREAKING CHANGE: Remove deprecated --runtime flag"
# This creates version 1.0.0
```

## Usage Examples

### Example 1: Add New Feature

```bash
# Make your changes
git add .
git commit -m "feat: add custom domain support for OIDC issuers"

# Check next version (dry-run)
make release-dry-run
# Output: Next version: v0.2.0 (current: v0.1.1)

# Create release
make release
# Creates tag v0.2.0

# Push to GitHub
git push origin v0.2.0
```

### Example 2: Bug Fix

```bash
git commit -m "fix: increase S3 deletion timeout to 5 minutes"

make release-dry-run
# Output: Next version: v0.1.2 (current: v0.1.1)

make release
git push origin v0.1.2
```

### Example 3: Multiple Commits

```bash
git commit -m "fix: handle missing private key file"
git commit -m "feat: add key rotation command"
git commit -m "docs: update architecture diagram"

make release-dry-run
# Output: Next version: v0.2.0 (current: v0.1.1)
# (Minor bump because of the feat: commit)

make release
git push origin v0.2.0
```

## Configuration

Version rules are defined in `.semver.yaml`:

```yaml
branches:
  - name: main

rules:
  major:
    - "BREAKING CHANGE"
  minor:
    - "feat"
  patch:
    - "fix"
    - "perf"
    - "refactor"
    - "chore"
    - "docs"

tag_prefix: "v"
```

## Installation

### Install go-semver-release

```bash
go install github.com/s0ders/go-semver-release@latest
```

Verify installation:

```bash
go-semver-release version
```

## Workflow

### Recommended Workflow

1. **Make changes** and commit with conventional commit messages
2. **Review commits** since last release:
   ```bash
   git log $(git describe --tags --abbrev=0)..HEAD --oneline
   ```
3. **Check next version** (dry-run):
   ```bash
   make release-dry-run
   ```
4. **Create release**:
   ```bash
   make release
   ```
5. **Push tag to trigger CI/CD**:
   ```bash
   git push origin $(git describe --tags --abbrev=0)
   ```

### CI/CD Integration

Tags pushed to GitHub can trigger automated workflows:

```yaml
# .github/workflows/release.yml
name: Release
on:
  push:
    tags:
      - "v*"
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Build Release
        run: make build
      - name: Create GitHub Release
        uses: softprops/action-gh-release@v1
        with:
          files: bin/rosactl
```

## FAQ

### Q: What if I forget to use conventional commits?

**A:** You can manually create a tag:

```bash
git tag v0.1.2
git push origin v0.1.2
```

But this defeats the purpose of automation. Try to use conventional commits going forward.

### Q: How do I see what commits contributed to a version?

**A:** Use git log with tag ranges:

```bash
# See commits between v0.1.0 and v0.1.1
git log v0.1.0..v0.1.1 --oneline

# See commits since last tag
git log $(git describe --tags --abbrev=0)..HEAD --oneline
```

### Q: Can I still use the old `make bump-version`?

**A:** Yes, it's still available as a fallback for manual version bumps, but `make release` is preferred for consistency.

### Q: What happens if no conventional commits are found?

**A:** `go-semver-release` will not create a new version. You need at least one commit with `feat:`, `fix:`, etc.

### Q: How do I create a pre-release version?

**A:** Configure a pre-release branch in `.semver.yaml`:

```yaml
branches:
  - name: main
  - name: rc
    prerelease: true # Creates v0.2.0-rc.1, v0.2.0-rc.2, etc.
```

## Reference

- [Semantic Versioning](https://semver.org/)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [go-semver-release GitHub](https://github.com/s0ders/go-semver-release)
