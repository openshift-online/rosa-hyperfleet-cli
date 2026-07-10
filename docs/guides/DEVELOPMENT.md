# Development Guide

## Getting Started

### Prerequisites

- **Go 1.25+** - Required for building from source
- **Make** - For using Makefile targets
- **Docker or Podman** - Optional, for LocalStack testing and building Lambda container images
- **Docker Compose** - Optional, for LocalStack testing
- **LocalStack Pro auth token** - Required to run Lambda container execution tests (set `LOCALSTACK_AUTH_TOKEN`)
- **AWS CLI** - Optional, for manual testing with real AWS
- **go-semver-release** - Optional, for semantic versioning

Install go-semver-release:

```bash
go install github.com/s0ders/go-semver-release@latest
```

### Setup

1. Clone the repository

```bash
git clone https://github.com/openshift-online/rosa-hyperfleet-cli.git
cd rosa-hyperfleet-cli
```

2. Install Go dependencies

```bash
go mod download
```

3. Build the project

```bash
make build
```

The binary will be available at `./bin/rosactl`.

4. Run tests

```bash
make test-localstack
```

## Development Workflow

### Branch Strategy

- `main` - Production-ready code, protected branch
- `feature/*` - New features and enhancements
- `fix/*` - Bug fixes
- `chore/*` - Maintenance tasks (dependencies, refactoring)
- `docs/*` - Documentation updates

### Making Changes

1. Create a feature branch from main

```bash
git checkout main
git pull origin main
git checkout -b feature/add-stack-outputs
```

2. Make your changes
3. Test locally with LocalStack

```bash
make test-localstack
```

4. Build and test the binary

```bash
make build
./rosactl cluster-vpc create test --region us-east-1 --help
```

5. Commit with conventional commit messages
6. Push and create a pull request

### Commit Messages

Follow the [conventional commits](https://www.conventionalcommits.org/) format for semantic versioning:

```
type(scope): subject

body

footer
```

**Types** (affects version bump):

- `feat`: New feature (minor version bump)
- `fix`: Bug fix (patch version bump)
- `docs`: Documentation only (patch version bump)
- `chore`: Maintenance task (patch version bump)
- `refactor`: Code refactoring (patch version bump)
- `test`: Adding tests (patch version bump)
- `BREAKING CHANGE`: Breaking API change (major version bump)

**Examples**:

```bash
# Feature (bumps 0.1.0 → 0.2.0)
git commit -m "feat: add VPC peering support"

# Bug fix (bumps 0.1.0 → 0.1.1)
git commit -m "fix: handle missing OIDC thumbprint gracefully"

# Documentation (bumps 0.1.0 → 0.1.1)
git commit -m "docs: update architecture diagrams"

# Breaking change (bumps 0.1.0 → 1.0.0)
git commit -m "feat: change cluster-iam API

BREAKING CHANGE: --oidc-issuer-url flag now requires https:// prefix"
```

See [docs/guides/VERSIONING.md](VERSIONING.md) for details.

## Code Style

### Formatting

Use `gofmt` for code formatting (enforced by Go toolchain):

```bash
gofmt -w .
```

### Linting

```bash
# Run golangci-lint (if configured)
golangci-lint run

# Basic vet
go vet ./...
```

### Best Practices

- Use `aws.String()`, `aws.Int32()` for pointer conversions
- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- Use typed errors for CloudFormation states (StackAlreadyExistsError, NoChangesError)
- Validate user input early (cluster name format, OIDC URL scheme)
- Use emoji sparingly in CLI output (only for major status indicators)
- Keep CloudFormation templates in YAML (readable and auditable)
- Tag all stacks with: Cluster, ManagedBy, red-hat-managed

## Testing

### Running Tests

```bash
# Run LocalStack integration tests
make test-localstack

# Run with verbose output using Ginkgo directly
go run github.com/onsi/ginkgo/v2/ginkgo -v test/localstack
```

### LocalStack Testing

LocalStack tests validate CLI commands and Lambda handler invocations against a local AWS environment.
Lambda container execution tests require LocalStack Pro.

1. Set the LocalStack Pro auth token (required for Lambda tests):

```bash
export LOCALSTACK_AUTH_TOKEN=your-token-here
```

2. Start LocalStack:

```bash
make localstack-up
```

3. Run tests:

```bash
make test-localstack
```

4. Stop LocalStack:

```bash
make localstack-down
```

**What the tests validate**:

- `cluster-vpc create` creates CloudFormation stack with VPC resources
- `cluster-vpc delete` deletes VPC stack
- `cluster-iam create` creates CloudFormation stack with IAM resources
- `cluster-iam delete` deletes IAM stack
- Stack events and outputs are properly returned
- Lambda `apply-cluster-vpc` / `delete-cluster-vpc` invocations create and delete VPC stacks
- Lambda `apply-cluster-iam` / `delete-cluster-iam` invocations create and delete IAM stacks
- Lambda deployment via `rosactl bootstrap create` with a container image

See [test/localstack/README.md](../../test/localstack/README.md) for details.

### Writing Tests

Use Ginkgo/Gomega for BDD-style tests:

```go
var _ = Describe("Feature", func() {
    It("should do something", func() {
        result := doSomething()
        Expect(result).To(Equal("expected"))
    })
})
```

For CLI command tests:

- Use `exec.CommandContext()` to invoke rosactl binary
- Use `gexec.Start()` to capture output
- Verify both stdout and stderr
- Check exit codes

## Debugging

### Verbose AWS SDK Logging

Set environment variables:

```bash
export AWS_SDK_LOG_LEVEL=debug
export AWS_SDK_LOG_MODE=LogWithHTTPBody
./rosactl cluster-vpc create test --region us-east-1
```

### CloudFormation Stack Events

View stack events for troubleshooting:

```bash
aws cloudformation describe-stack-events \
  --stack-name rosa-my-cluster-vpc \
  --region us-east-1
```

### Common Issues

- **"Template not found"**: Templates are embedded - rebuild binary if templates changed
- **"Stack already exists"**: CLI automatically attempts update - check stack status
- **"Insufficient permissions"**: Verify AWS credentials have CloudFormation, EC2, IAM permissions
- **"OIDC thumbprint fetch failed"**: Ensure OIDC URL is publicly accessible over HTTPS

## Building

### Local Build

```bash
# Build for current platform
make build

# Output: ./rosactl
```

### Build for Specific Platform

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o rosactl cmd/rosactl/main.go

# macOS
GOOS=darwin GOARCH=arm64 go build -o rosactl cmd/rosactl/main.go

# Windows
GOOS=windows GOARCH=amd64 go build -o rosactl.exe cmd/rosactl/main.go
```

### Release Build

```bash
# Check what version would be released
make release-dry-run

# Create semantic version tag
make release

# Push tag to GitHub
git push origin v0.2.0
```

### Docker Build (for Lambda deployment)

```bash
# Build container image
docker build -f Dockerfile -t rosactl:latest .

# Tag for ECR
docker tag rosactl:latest <account>.dkr.ecr.us-east-1.amazonaws.com/rosactl:latest

# Push to ECR
aws ecr get-login-password --region us-east-1 | \
  docker login --username AWS --password-stdin <account>.dkr.ecr.us-east-1.amazonaws.com
docker push <account>.dkr.ecr.us-east-1.amazonaws.com/rosactl:latest
```

## Project Structure

```text
rosa-hyperfleet-cli/
├── cmd/rosactl/                     # Entry point (main.go)
├── internal/
│   ├── commands/                    # CLI command groups
│   │   ├── bootstrap/               # Lambda bootstrap deployment
│   │   ├── clustervpc/              # VPC management
│   │   ├── clusteriam/              # IAM management
│   │   ├── handler/                 # Lambda handler entrypoint command
│   │   └── version/                 # Version command
│   ├── services/                    # Shared business logic (used by CLI and Lambda)
│   │   ├── clustervpc/              # VPC service (CreateVPC, DeleteVPC)
│   │   └── clusteriam/              # IAM service (CreateIAM, DeleteIAM)
│   ├── aws/
│   │   └── cloudformation/          # CloudFormation client
│   ├── cloudformation/
│   │   └── templates/               # Embedded templates (go:embed)
│   │       ├── cluster-vpc.yaml
│   │       ├── cluster-iam.yaml
│   │       └── lambda-bootstrap.yaml
│   ├── crypto/                      # TLS thumbprint utilities
│   └── lambda/                      # Lambda event handler (optional)
├── test/
│   └── localstack/                  # LocalStack integration tests
│       ├── localstack_suite_test.go
│       ├── localstack_test.go       # CLI tests
│       └── lambda_test.go           # Lambda handler invocation tests
├── docker-compose.localstack.yaml   # LocalStack Pro compose file
└── docs/
    ├── architecture/                # Architecture docs
    ├── guides/                      # User and developer guides
    └── specs/                       # Feature specifications
```

## Adding New Features

### Adding a New CLI Command

1. Create command directory in `internal/commands/`
2. Implement `New<Command>Command()` function
3. Register in root command (`cmd/rosactl/main.go`)
4. Add tests in LocalStack test suite

Example:

```go
// internal/commands/myfeature/myfeature.go
package myfeature

import "github.com/spf13/cobra"

func NewCommand() *cobra.Command {
    return &cobra.Command{
        Use:   "my-feature",
        Short: "Manage my feature",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Implementation
            return nil
        },
    }
}
```

### Adding a New CloudFormation Template

1. Create template in `internal/cloudformation/templates/`
2. Template is automatically embedded via `//go:embed *.yaml`
3. Read template using `templates.Read("my-template.yaml")`
4. Rebuild binary to pick up new template

## Documentation

### Documentation Structure

- `README.md` - Main project documentation
- `docs/architecture/ARCHITECTURE.md` - System architecture and components
- `docs/guides/DEVELOPMENT.md` - This file
- `docs/guides/VERSIONING.md` - Semantic versioning guide
- `test/localstack/README.md` - LocalStack testing guide

### Updating Documentation

When making changes that affect user-facing behavior or architecture:

1. Update README.md if adding/changing CLI commands
2. Update ARCHITECTURE.md if changing system design or architectural decisions
3. Update DEVELOPMENT.md for development workflow changes
4. Update inline code documentation with `//` comments

### Documentation Best Practices

- Keep examples up-to-date with current CLI syntax
- Use actual command output in examples (not made-up placeholders)
- Document breaking changes in commit messages
- Update architecture diagrams when components change
- Link related documentation sections

## Contributing

### Pull Request Process

1. Fork the repository
2. Create a feature branch from `main`
3. Make changes with conventional commit messages
4. Run tests locally (`make test-localstack`)
5. Build binary and test manually
6. Push branch and create pull request
7. Address review feedback
8. PR gets merged to `main`

### Code Review Guidelines

**For Reviewers**:

- Check for security issues (command injection, path traversal)
- Verify CloudFormation templates are valid
- Ensure error messages are helpful
- Confirm tests cover new functionality
- Check that documentation is updated

**For Contributors**:

- Keep changes focused and atomic
- Write clear commit messages
- Add tests for new features
- Update documentation
- Respond to review feedback promptly

## Resources

- [AWS CloudFormation Documentation](https://docs.aws.amazon.com/cloudformation/)
- [AWS SDK for Go v2](https://aws.github.io/aws-sdk-go-v2/docs/)
- [Cobra CLI Framework](https://github.com/spf13/cobra)
- [Ginkgo Testing Framework](https://onsi.github.io/ginkgo/)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [go-semver-release](https://github.com/s0ders/go-semver-release)
