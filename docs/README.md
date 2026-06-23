# Project Documentation

Welcome to the rosactl documentation! This directory contains all project documentation, architecture guides, and references.

## 📚 Quick Links

| What do you want to do?         | Start here                                                   |
| ------------------------------- | ------------------------------------------------------------ |
| **Understand the architecture** | [architecture/ARCHITECTURE.md](architecture/ARCHITECTURE.md) |
| **Build and contribute**        | [guides/DEVELOPMENT.md](guides/DEVELOPMENT.md)               |
| **Manage versions**             | [guides/VERSIONING.md](guides/VERSIONING.md)                 |
| **Run integration tests**       | [../test/localstack/README.md](../test/localstack/README.md) |

## 📁 Documentation Structure

```
docs/
├── README.md                    # This file
├── architecture/                # System design and architecture
│   └── ARCHITECTURE.md          # System architecture overview
├── guides/                      # User and developer guides
│   ├── DEVELOPMENT.md           # Developer setup and workflows
│   ├── VERSIONING.md            # Semantic versioning guide
│   └── DOCUMENTATION.md         # Documentation writing guidelines
└── specs/                       # Feature specifications and references
    ├── reference-gist-1.md      # OIDC/STS authentication flow reference
    └── references.md            # External project references
```

## 🏗️ Architecture Documentation

### [ARCHITECTURE.md](architecture/ARCHITECTURE.md)

Complete system architecture including:

- Direct CloudFormation management with optional Lambda
- Managed OIDC architecture (Red Hat-hosted)
- Dual-mode Go binary (CLI and Lambda)
- Embedded CloudFormation templates
- Security architecture and trust chains
- Design trade-offs and architectural decisions

## 📖 User & Developer Guides

### [DEVELOPMENT.md](guides/DEVELOPMENT.md)

Developer setup and contribution guide:

- Local development environment setup
- Build and test instructions
- Code organization and patterns
- Pull request workflow

### [VERSIONING.md](guides/VERSIONING.md)

Semantic versioning with conventional commits:

- How to use `make release` for version management
- Conventional commit message format
- Version bump rules (feat, fix, BREAKING CHANGE)
- Release workflow

### [DOCUMENTATION.md](guides/DOCUMENTATION.md)

Documentation writing guidelines:

- Documentation standards and style
- How to write effective docs
- Examples and anti-patterns

## 📋 Specifications & References

### Feature Specifications

| Document                                           | Purpose                                    |
| -------------------------------------------------- | ------------------------------------------ |
| [LocalStack Testing](../test/localstack/README.md) | LocalStack integration testing with Ginkgo |

### References

| Document                                         | Purpose                                                                    |
| ------------------------------------------------ | -------------------------------------------------------------------------- |
| [reference-gist-1.md](specs/reference-gist-1.md) | Deep dive into OIDC/STS cross-account authentication in ROSA HCP           |
| [references.md](specs/references.md)             | External project references (hypershift, rosa, rosa-regional-platform-api) |

## 🚀 Getting Started

### For New Users

1. Read [ARCHITECTURE.md](architecture/ARCHITECTURE.md) to understand the system
2. Check [../README.md](../README.md) for installation and quick start
3. Review the main README for command reference

### For Contributors

1. Read [DEVELOPMENT.md](guides/DEVELOPMENT.md) for setup instructions
2. Review [ARCHITECTURE.md](architecture/ARCHITECTURE.md) to understand the system
3. Follow [VERSIONING.md](guides/VERSIONING.md) for commit message format
4. Check [DOCUMENTATION.md](guides/DOCUMENTATION.md) for documentation standards

### For Testing

1. Read [LocalStack Testing Guide](../test/localstack/README.md) for integration test setup
2. Run `make test-localstack` to execute tests against LocalStack
3. Review [DEVELOPMENT.md](guides/DEVELOPMENT.md) for testing best practices

## 🔄 Documentation Lifecycle

### Creating New Documentation

1. **Choose the right location**:
   - Architecture design → `architecture/`
   - User/developer guide → `guides/`
   - Feature spec or reference → `specs/`

2. **Follow the style guide**: See [DOCUMENTATION.md](guides/DOCUMENTATION.md)

3. **Update this index**: Add your new doc to this README

4. **Use conventional commits**:
   ```bash
   git commit -m "docs: add VPC deployment guide"
   ```

### Updating Existing Documentation

When the code changes:

1. **Update affected docs** immediately (don't let docs drift)
2. **Test all examples** (ensure commands still work)
3. **Update version badges** if needed (see [VERSIONING.md](guides/VERSIONING.md))
4. **Review for accuracy** before committing

### Documentation Review Checklist

Before merging documentation changes:

- [ ] Spell-checked (no typos)
- [ ] Links work (all references valid)
- [ ] Code examples tested (commands actually work)
- [ ] Output current (matches actual tool behavior)
- [ ] Diagrams render correctly
- [ ] Added to this index (if new file)

## 🤝 Contributing to Documentation

Documentation improvements are always welcome! To contribute:

1. **Fix typos and errors**: Just submit a PR
2. **Add examples**: Include working code and expected output
3. **New features**: Update both code and docs in the same PR
4. **Improve clarity**: Simplify complex explanations

See [DOCUMENTATION.md](guides/DOCUMENTATION.md) for detailed guidelines.

## 📝 Documentation Conventions

### File Naming

- Use `UPPERCASE.md` for top-level docs (ARCHITECTURE.md, README.md)
- Use `lowercase-with-dashes.md` for feature specs (feature-e2e.md)
- Use descriptive names for references (reference-gist-1.md)

### Markdown Style

- Use ATX-style headers (`#` not underlines)
- Fenced code blocks with language hints (`bash not `)
- Tables for comparisons and structured data
- Emoji for visual categorization (📚 📁 🚀 etc.)

### Code Examples

- Always include expected output
- Use real commands that actually work
- Show both success and error cases
- Include comments explaining non-obvious steps

## 🔗 External Resources

- [Main README](../README.md) - Project README
- [LocalStack Testing Guide](../test/localstack/README.md) - Integration testing guide
- [Makefile](../Makefile) - Build targets and commands
- [GitHub Repository](https://github.com/openshift-online/rosa-regional-platform-cli)
- [ROSA Regional Platform Terraform](https://github.com/openshift-online/rosa-regional-platform) - Reference implementation
- [HyperShift](https://github.com/openshift/hypershift) - OIDC implementation reference

---

**Questions or suggestions?** Open an issue or submit a PR!
