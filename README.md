# rosactl

CLI tool for managing AWS infrastructure and cluster lifecycle for ROSA hosted clusters.

Manages VPC networking, IAM roles, OIDC providers, cluster creation, and node pools. All AWS infrastructure is deployed via CloudFormation stacks; cluster and node pool operations go through the platform API.

## Quick Start

```bash
# Build
git clone https://github.com/openshift-online/rosa-hyperfleet-cli.git
cd rosa-hyperfleet-cli
make build

# Typical workflow
rosactl login --url https://api.platform.example.com
rosactl cluster-vpc create my-cluster --region us-east-1
rosactl cluster-iam create my-cluster --region us-east-1
rosactl cluster create my-cluster --region us-east-1
rosactl cluster-oidc create my-cluster \
  --oidc-issuer-url <issuer-url-from-cluster-create> --region us-east-1
rosactl nodepool create my-np --cluster-id <cluster-id> --region us-east-1
rosactl cluster kubeconfig my-cluster > ~/.kube/my-cluster

# Teardown (reverse order)
rosactl nodepool delete <nodepool-id>
rosactl cluster-oidc delete my-cluster --region us-east-1
rosactl cluster-iam delete my-cluster --region us-east-1
rosactl cluster-vpc delete my-cluster --region us-east-1
```

## Configuration

### AWS Credentials

rosactl uses the AWS default credential chain:

```bash
# Option 1: Credentials file (~/.aws/credentials)
[default]
aws_access_key_id = YOUR_ACCESS_KEY
aws_secret_access_key = YOUR_SECRET_KEY
region = us-east-1

# Option 2: Environment variables
export AWS_ACCESS_KEY_ID=YOUR_ACCESS_KEY
export AWS_SECRET_ACCESS_KEY=YOUR_SECRET_KEY
export AWS_REGION=us-east-1

# Option 3: Named profile
export AWS_PROFILE=your-profile-name
# or pass --profile your-profile-name to any command
```

### Platform API

```bash
rosactl login --url https://api.platform.example.com
```

Stores the base URL in `~/.config/rosactl/config.json`. Required for `cluster` and `nodepool` commands.

### Global Flags

| Flag | Description |
|---|---|
| `--region` | AWS region |
| `--profile` | AWS named profile |
| `-v`, `--verbose` | Enable verbose output |

## Commands

### login

```bash
rosactl login --url <platform-api-url>
```

### cluster

#### create

Create a cluster configuration from CloudFormation stacks and submit to the platform API.

```bash
# Generate config from stacks and submit
rosactl cluster create my-cluster --region us-east-1

# Dry-run: generate config without submitting
rosactl cluster create my-cluster --region us-east-1 --dry-run
rosactl cluster create my-cluster --region us-east-1 --dry-run --output-file my-cluster.json

# Submit an existing payload file
rosactl cluster create my-cluster --region us-east-1 --payload my-cluster.json

# Override management cluster placement
rosactl cluster create my-cluster --region us-east-1 --payload my-cluster.json --placement mgmt-cluster-01
```

| Flag | Default | Description |
|---|---|---|
| `--region` | `us-east-1` | AWS region |
| `--dry-run` | `false` | Generate config only, do not submit |
| `--output-file` | | Output file for config (defaults to `<name>-cluster.json` in dry-run) |
| `--payload` | | JSON payload file to POST (mutually exclusive with `--dry-run`) |
| `--placement` | | Management cluster name |
| `--version` | `4.22` | OpenShift version |
| `--compute-replicas` | `3` | Number of compute replicas |
| `--compute-machine-type` | `m5.xlarge` | EC2 instance type for compute |
| `--multi-az` | `true` | Multi-AZ deployment |
| `--provider` | `aws` | Cloud provider |
| `--target-project-id` | | Target project ID |
| `--label-environment` | `dev` | Environment label |
| `--label-team` | `platform` | Team label |
| `--output` | | Output format (`json`) |

#### list

```bash
rosactl cluster list
rosactl cluster list --status Ready --limit 10
rosactl cluster list --output json
```

| Flag | Default | Description |
|---|---|---|
| `--limit` | `50` | Max clusters to return (1-100) |
| `--offset` | `0` | Number of clusters to skip |
| `--status` | | Filter: Pending, Progressing, Ready, Failed |
| `-o`, `--output` | `table` | Output format: `table` or `json` |

#### kubeconfig

```bash
rosactl cluster kubeconfig <cluster-id|cluster-name> > ~/.kube/my-cluster
kubectl --kubeconfig=~/.kube/my-cluster get nodes
```

Uses rosactl as a kubectl exec credential plugin for AWS IAM authentication.

#### get-token

```bash
rosactl cluster get-token --cluster-id <cluster-id>
```

Generates a presigned STS token for kubectl authentication. Used internally by the kubeconfig exec credential plugin.

### cluster-vpc

#### create

```bash
rosactl cluster-vpc create my-cluster --region us-east-1

# Custom CIDR and availability zones
rosactl cluster-vpc create my-cluster \
  --region us-east-1 \
  --vpc-cidr 10.1.0.0/16 \
  --availability-zones us-east-1a,us-east-1b \
  --single-nat-gateway=false
```

| Flag | Default | Description |
|---|---|---|
| `--region` | | AWS region (required) |
| `--vpc-cidr` | `10.0.0.0/16` | VPC CIDR block |
| `--public-subnet-cidrs` | `10.0.101.0/24,10.0.102.0/24,10.0.103.0/24` | Public subnet CIDRs |
| `--private-subnet-cidrs` | `10.0.0.0/19,10.0.32.0/19,10.0.64.0/19` | Private subnet CIDRs |
| `--availability-zones` | | AZ names, e.g. `us-east-1a,us-east-1b` (auto-detected if empty) |
| `--single-nat-gateway` | `true` | Single NAT gateway (cost savings) vs per-AZ (HA) |

#### list / describe / delete

```bash
rosactl cluster-vpc list --region us-east-1
rosactl cluster-vpc describe my-cluster --region us-east-1
rosactl cluster-vpc delete my-cluster --region us-east-1
```

### cluster-iam

#### create

```bash
# Create IAM roles (OIDC provider added separately later)
rosactl cluster-iam create my-cluster --region us-east-1

# Create IAM roles + OIDC provider in one step
rosactl cluster-iam create my-cluster \
  --oidc-issuer-url https://oidc.example.com/my-cluster \
  --region us-east-1
```

| Flag | Default | Description |
|---|---|---|
| `--region` | | AWS region (required) |
| `--oidc-issuer-url` | | Also creates OIDC provider if provided |

#### list / describe / delete

```bash
rosactl cluster-iam list --region us-east-1
rosactl cluster-iam describe my-cluster --region us-east-1
rosactl cluster-iam delete my-cluster --region us-east-1
```

### cluster-oidc

Manages the IAM OIDC provider separately from IAM roles. Use when the OIDC issuer URL is not known at IAM creation time (e.g., it comes from `cluster create` output).

#### create

```bash
rosactl cluster-oidc create my-cluster \
  --oidc-issuer-url https://oidc.example.com/my-cluster \
  --region us-east-1
```

| Flag | Default | Description |
|---|---|---|
| `--oidc-issuer-url` | | OIDC issuer URL (required) |
| `--oidc-thumbprint` | | TLS thumbprint (auto-fetched if omitted) |
| `--region` | | AWS region (required) |

Creates a CloudFormation stack (`rosa-{name}-oidc`) and updates the IAM roles stack trust policies.

#### list / delete

```bash
rosactl cluster-oidc list --region us-east-1
rosactl cluster-oidc delete my-cluster --region us-east-1
```

### nodepool

#### create

```bash
rosactl nodepool create my-np --cluster-id <cluster-id> --region us-east-1

# With explicit settings
rosactl nodepool create my-np \
  --cluster-id <cluster-id> \
  --replicas 3 \
  --instance-type m5.2xlarge \
  --region us-east-1
```

| Flag | Default | Description |
|---|---|---|
| `--cluster-id` | | Cluster ID (required) |
| `--replicas` | `2` | Number of worker replicas |
| `--instance-type` | `m6a.xlarge` | EC2 instance type |
| `--subnet-id` | | Subnet ID (auto-discovered from cluster if omitted) |
| `--instance-profile` | | IAM instance profile (auto-discovered if omitted) |
| `--security-groups` | | Security group IDs (auto-discovered if omitted) |
| `-o`, `--output` | | Output format (`json`) |

#### list

```bash
rosactl nodepool list --cluster-id <cluster-id>
rosactl nodepool list --cluster-id <cluster-id> --output json
```

| Flag | Default | Description |
|---|---|---|
| `--cluster-id` | | Cluster ID (required) |
| `--limit` | `50` | Max results (1-100) |
| `--offset` | `0` | Number to skip |
| `-o`, `--output` | `table` | Output format: `table` or `json` |

#### delete

```bash
rosactl nodepool delete <nodepool-id>
```

### bootstrap (optional)

Deploy rosactl as a Lambda function for event-driven workflows. Not required for normal CLI usage.

```bash
# Deploy
rosactl bootstrap create \
  --image-uri <account>.dkr.ecr.us-east-1.amazonaws.com/rosactl:latest \
  --region us-east-1

# Check status
rosactl bootstrap status --region us-east-1

# Remove
rosactl bootstrap delete --region us-east-1
```

| Flag | Default | Description |
|---|---|---|
| `--image-uri` | | ECR container image URI (required for create) |
| `--function-name` | `rosa-regional-platform-lambda` | Lambda function name |
| `--stack-name` | `rosa-regional-platform-lambda` | CloudFormation stack name |
| `--region` | | AWS region (required) |

The Lambda accepts JSON event payloads with an `action` field: `apply-cluster-vpc`, `delete-cluster-vpc`, `apply-cluster-iam`, `delete-cluster-iam`.

### version

```bash
rosactl version
```

## CloudFormation Stacks

### Naming Convention

| Stack | Name |
|---|---|
| VPC | `rosa-{cluster-name}-vpc` |
| IAM | `rosa-{cluster-name}-iam` |
| OIDC | `rosa-{cluster-name}-oidc` |

All stacks are tagged with `Cluster`, `ManagedBy: rosactl`, and `red-hat-managed: true`.

### VPC Resources

The VPC stack creates:

- VPC with configurable CIDR (default `10.0.0.0/16`)
- Public subnets (1-3, across availability zones)
- Private subnets (1-3, for worker nodes)
- Internet Gateway
- NAT Gateway(s) — single or per-AZ
- Route tables and routes
- Worker node security group
- Route53 private hosted zone (`{cluster}.hypershift.local`)

### IAM Resources

The IAM stack creates:

- 7 control plane roles: Ingress Operator, Kube Controller Manager, EBS CSI Driver Operator, Image Registry Operator, Cloud Network Config Operator, Control Plane Operator, Node Pool Management
- Worker node IAM role and instance profile
- (Optional) IAM OIDC provider — if `--oidc-issuer-url` is provided, or via the separate `cluster-oidc create` command

All roles use OIDC federation with minimal required permissions. The OIDC thumbprint is auto-fetched from the issuer URL.

## Development

### Build and Test

```bash
make build                 # Build ./bin/rosactl
make test                  # Unit tests (go test -race)
make test-localstack       # Integration tests (requires LocalStack Pro + LOCALSTACK_AUTH_TOKEN)
make verify                # fmt-check + vet + lint
make fmt                   # Auto-format
make clean                 # Remove build artifacts
```

### Release

```bash
# Conventional commits drive versioning
git commit -m "feat: add new feature"    # minor bump
git commit -m "fix: bug fix"             # patch bump

make release-dry-run    # Preview next version
make release            # Create release tag
git push origin v0.2.0
```

See [docs/guides/VERSIONING.md](docs/guides/VERSIONING.md) for details.

### Project Structure

```
rosa-hyperfleet-cli/
├── cmd/rosactl/                     # Entry point
├── internal/
│   ├── commands/                    # CLI commands (Cobra)
│   │   ├── bootstrap/               # Lambda bootstrap
│   │   ├── cluster/                 # Cluster lifecycle (create, list, kubeconfig, get-token)
│   │   ├── clusteriam/              # IAM roles (create, delete, list, describe)
│   │   ├── clusteroidc/             # OIDC provider (create, delete, list)
│   │   ├── clustervpc/              # VPC networking (create, delete, list, describe)
│   │   ├── handler/                 # Lambda handler (hidden)
│   │   ├── login/                   # Platform API login
│   │   ├── nodepool/                # Node pools (create, list, delete)
│   │   └── version/                 # Version info
│   ├── services/                    # Business logic (shared by CLI and Lambda)
│   ├── aws/cloudformation/          # CloudFormation client
│   ├── cloudformation/templates/    # Embedded CloudFormation templates (go:embed)
│   ├── crypto/                      # TLS thumbprint utilities
│   └── lambda/                      # Lambda event handler
├── test/localstack/                 # Integration tests
├── docs/                            # Architecture, guides, specs
└── Makefile
```

## Architecture

See [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md) for full details.

```
┌─────────────────────────────────────────────────────┐
│              rosactl CLI / Lambda Handler            │
│                  (Cobra Framework)                   │
└────────────────────────┬────────────────────────────┘
                         │
    ┌────────┬───────────┼──────────┬──────────┐
    │        │           │          │          │
┌───▼──┐ ┌──▼───┐ ┌─────▼────┐ ┌──▼───┐ ┌────▼─────┐
│ VPC  │ │ IAM  │ │  OIDC    │ │Cluster│ │ NodePool │
│Cmds  │ │ Cmds │ │  Cmds    │ │ Cmds  │ │  Cmds    │
└──┬───┘ └──┬───┘ └────┬─────┘ └──┬───┘ └────┬─────┘
   │        │          │          │           │
   └────────┼──────────┘          └─────┬─────┘
            │                           │
   ┌────────▼────────────┐    ┌─────────▼──────────┐
   │   Service Layer     │    │   Platform API      │
   │ clustervpc/iam/oidc │    │ cluster / nodepool  │
   └────────┬────────────┘    └────────────────────┘
            │
   ┌────────▼────────────┐
   │ CloudFormation Client│
   │  (Embedded Templates)│
   └────────┬─────────────┘
            │
   ┌────────▼────────────┐
   │  AWS CloudFormation  │
   │ VPC | IAM | EC2 | R53│
   └──────────────────────┘
```

## Troubleshooting

**"Stack already exists"** — The command automatically attempts to update the existing stack. If stuck in a failed state, delete and recreate.

**"Failed to fetch TLS thumbprint"** — Ensure the OIDC issuer URL is publicly accessible over HTTPS with a valid TLS certificate.

**"Insufficient permissions"** — Check CloudFormation stack events: `aws cloudformation describe-stack-events --stack-name rosa-my-cluster-vpc`. Required permissions are listed in the [Prerequisites](#prerequisites) section below.

**NAT Gateway timeout (LocalStack)** — Expected; LocalStack has limited NAT Gateway support. Tests accept both CREATE_COMPLETE and CREATE_FAILED.

**Lambda container fails (LocalStack)** — Requires LocalStack Pro. Set `LOCALSTACK_AUTH_TOKEN` before starting LocalStack.

## Prerequisites

- **Go 1.25+** (building from source)
- **AWS credentials** (see [Configuration](#configuration))
- **AWS IAM permissions**:
  - CloudFormation: `CreateStack`, `UpdateStack`, `DeleteStack`, `DescribeStacks`, `ListStacks`, `DescribeStackEvents`, `DescribeStackResources`, `ListStackResources`
  - EC2: `CreateVpc`, `DeleteVpc`, `CreateSubnet`, `DeleteSubnet`, `CreateSecurityGroup`, `DeleteSecurityGroup`, `CreateNatGateway`, `DeleteNatGateway`, `CreateInternetGateway`, `DeleteInternetGateway`, `CreateRoute`, `DeleteRoute`, `CreateRouteTable`, `DeleteRouteTable`, `AuthorizeSecurityGroupEgress`, `AuthorizeSecurityGroupIngress`
  - IAM: `CreateRole`, `DeleteRole`, `AttachRolePolicy`, `DetachRolePolicy`, `CreateInstanceProfile`, `DeleteInstanceProfile`, `AddRoleToInstanceProfile`, `RemoveRoleFromInstanceProfile`, `CreateOpenIDConnectProvider`, `DeleteOpenIDConnectProvider`, `GetOpenIDConnectProvider`, `ListOpenIDConnectProviders`
  - Route53: `CreateHostedZone`, `DeleteHostedZone`
- **Optional**: [go-semver-release](https://github.com/s0ders/go-semver-release) for versioning, [LocalStack Pro](https://localstack.cloud) for integration tests

## Documentation

- [Architecture](docs/architecture/ARCHITECTURE.md)
- [Versioning Guide](docs/guides/VERSIONING.md)
- [Development Guide](docs/guides/DEVELOPMENT.md)
- [LocalStack Testing](test/localstack/README.md)

## License

Apache License 2.0
