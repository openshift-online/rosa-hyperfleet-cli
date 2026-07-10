# Documentation Guidelines

This guide defines the standards and best practices for writing documentation in the rosactl project.

## Core Principles

### 1. Conciseness

- **Be direct and concise** - Remove unnecessary words
- **One concept per paragraph** - Don't mix multiple ideas
- **Use active voice** - "The CLI invokes the Lambda" not "The Lambda is invoked by the CLI"

### 2. Visual Over Text

- **Prioritize diagrams** - Use ASCII diagrams, flowcharts, and swimlanes
- **Show, don't tell** - Include code examples and command outputs
- **Tables over lists** - Use tables for comparisons and structured data

### 3. Examples First

- **Start with examples** - Show working code before explaining theory
- **Real-world scenarios** - Use actual use cases, not abstract examples
- **Include expected output** - Always show what the user should see

## Documentation Types

### Architecture Documentation (`docs/architecture/`)

**Purpose**: Explain system design and structure

**Format**:

```markdown
# Component Name

## Overview

[1-2 sentence description]

## Architecture Diagram

[ASCII or Mermaid diagram]

## Key Components

- Component A: [Purpose]
- Component B: [Purpose]

## Data Flow

[Swimlane or sequence diagram]

## Trade-offs

[Design decisions and rationale]
```

**Examples**:

- `ARCHITECTURE.md` - System-wide architecture and design decisions

---

### User Guides (`docs/guides/`)

**Purpose**: Help users accomplish specific tasks

**Format**:

````markdown
# Task Name

## Quick Start

[30-second example]

## Detailed Steps

1. Step 1
   ```bash
   command example
   ```
````

Expected output:

```
output example
```

2. Step 2
   ...

## Common Issues

[Troubleshooting tips]

````

**Examples**:
- `VERSIONING.md` - How to manage versions
- `DEVELOPMENT.md` - Developer setup guide

---

### Feature Specifications (`docs/specs/`)

**Purpose**: Define requirements and implementation details

**Format**:
```markdown
# Feature Name

## Summary
[What this feature does in 2-3 sentences]

## User Stories
- As a [role], I want [feature] so that [benefit]

## Requirements
- MUST: [Critical requirement]
- SHOULD: [Important requirement]
- COULD: [Nice-to-have]

## Implementation
[Technical approach]

## Examples
[Usage examples]
````

**Examples**:

- `feature-e2e.md` - End-to-end testing

---

## Writing Style

### Commands and Code

✅ **Good**:

````markdown
Bootstrap the Lambda function:

```bash
rosactl bootstrap create --image-uri 123456789012.dkr.ecr.us-east-1.amazonaws.com/rosa-cli:latest --region us-east-1
```
````

Output:

```
Creating CloudFormation stack: rosactl-bootstrap
Lambda function created successfully!
ARN: arn:aws:lambda:us-east-1:123456789012:function:rosactl-lambda
```

````

❌ **Bad**:
```markdown
You can bootstrap the Lambda infrastructure by running the command with the container image URI.
````

### Diagrams

✅ **Good** (Swimlane diagram):

```markdown
User CLI CloudFormation Lambda IAM
| | | | |
|-- create cmd --->| | | |
| |-- apply CF --->| | |
| | |-- invoke --->| |
| | | |-- create ---->|
| | | |<-- ARN -------|
|<--- outputs -----| | | |
```

❌ **Bad** (Wall of text):

```markdown
When the user runs the create command, the CLI applies a CloudFormation stack which invokes the Lambda function, and the Lambda creates IAM resources and returns their ARNs.
```

### Comparisons

✅ **Good** (Table):

```markdown
| Command            | Purpose              | Use Case       |
| ------------------ | -------------------- | -------------- |
| bootstrap create   | Deploy Lambda        | One-time setup |
| cluster-iam create | Create IAM resources | Per cluster    |
| cluster-iam delete | Remove IAM resources | Cleanup        |
```

❌ **Bad** (Long paragraphs):

```markdown
The bootstrap create command deploys the Lambda function and is used for one-time setup. The cluster-iam create command creates IAM resources for each cluster. The cluster-iam delete command removes IAM resources during cleanup.
```

## Documentation Checklist

Before committing documentation, verify:

- [ ] **Spell-checked** - No typos (use `aspell` or IDE spellchecker)
- [ ] **Links work** - All internal references point to existing files
- [ ] **Code examples tested** - All commands actually work
- [ ] **Output current** - Example outputs match actual tool behavior
- [ ] **Diagrams clear** - ASCII diagrams render correctly in monospace font
- [ ] **Navigation clear** - Easy to find related docs
- [ ] **No dead ends** - Each doc links to related docs

## File Organization

```
docs/
├── README.md                    # Documentation index
├── architecture/
│   └── ARCHITECTURE.md          # System architecture and design decisions
├── guides/
│   ├── VERSIONING.md            # How to version releases
│   ├── DEVELOPMENT.md           # Developer setup
│   └── DOCUMENTATION.md         # This file
└── specs/
    ├── feature-e2e.md           # E2E testing spec
    ├── reference-gist-1.md      # OIDC/STS flow reference
    └── references.md            # External references
```

## Commit Messages for Documentation

Follow conventional commits:

```bash
# Documentation updates
git commit -m "docs: update cluster IAM guide with examples"

# New documentation
git commit -m "docs: add CloudFormation troubleshooting guide"

# Fix typos
git commit -m "docs: fix typos in architecture documentation"
```

## Common Mistakes to Avoid

### 1. Too Much Theory, Not Enough Practice

❌ **Bad**:

> "The IAM OIDC provider uses CloudFormation-based declarative infrastructure to establish federated trust relationships with the managed OIDC issuer in the Red Hat control plane..."

✅ **Good**:

```bash
# Create cluster IAM resources
rosactl cluster-iam create my-cluster \
  --oidc-issuer-url https://d1234.cloudfront.net/my-cluster \
  --region us-east-1

# What this creates:
# - IAM OIDC Provider (points to Red Hat's CloudFront)
# - 7 control plane IAM roles (for operators)
# - Worker node IAM role + instance profile
```

### 2. Missing Context

❌ **Bad**:

```bash
rosactl bootstrap create --image-uri <uri> --region us-east-1
```

✅ **Good**:

```bash
# First, push the container image to ECR
docker build -f Dockerfile -t rosa-cli:latest .
docker tag rosa-cli:latest 123456789012.dkr.ecr.us-east-1.amazonaws.com/rosa-cli:latest
docker push 123456789012.dkr.ecr.us-east-1.amazonaws.com/rosa-cli:latest

# Then bootstrap the Lambda infrastructure (one-time per region)
rosactl bootstrap create \
  --image-uri 123456789012.dkr.ecr.us-east-1.amazonaws.com/rosa-cli:latest \
  --region us-east-1
```

### 3. Outdated Examples

❌ **Bad** (references deleted commands):

```bash
rosactl oidc create my-cluster  # This command no longer exists
```

✅ **Good** (current syntax):

```bash
rosactl cluster-iam create my-cluster \
  --oidc-issuer-url https://d1234.cloudfront.net/my-cluster \
  --region us-east-1
```

## References

- [Google Developer Documentation Style Guide](https://developers.google.com/style)
- [Kubernetes Documentation Style Guide](https://kubernetes.io/docs/contribute/style/style-guide/)
- [The Documentation System](https://documentation.divio.com/) - Four types of docs framework
