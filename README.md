# rosactl

A command-line tool for ROSA HyperFleet

Manages AWS infrastructure for ROSA hosted clusters including VPC networking, IAM roles, and OIDC providers via CloudFormation stacks.

![Version](https://img.shields.io/badge/version-0.1.0-blue.svg)

## Features

### Cluster Infrastructure Management

- **VPC Networking**: Create and manage VPCs, subnets, NAT gateways, and security groups for hosted clusters
- **IAM Resources**: Create OIDC providers and IAM roles for cluster control plane and worker nodes
- **CloudFormation-based**: All resources deployed via CloudFormation stacks for consistency and rollback support
- **Embedded templates**: CloudFormation templates embedded in binary using go:embed
- **Direct execution**: No Lambda bootstrap required for basic operations

### Optional Lambda Bootstrap

- **Container-based Lambda**: Deploy rosactl as a Lambda function for event-driven workflows
- **Automated deployments**: Integrate with CI/CD pipelines and AWS event sources
- **Same binary**: Lambda uses the same rosactl binary packaged in a container

### Developer Experience

- **Semantic versioning**: Automated version management with conventional commits
- **LocalStack testing**: Integration tests against LocalStack for CloudFormation validation
- **Clear error messages**: User-friendly error reporting with CloudFormation event details
- **Extensive documentation**: Architecture docs, guides, and examples

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/openshift-online/rosa-hyperfleet-cli.git
cd rosa-hyperfleet-cli

# Build
make build

# Install globally (optional)
make install
```

### Basic Usage

```bash
# Create VPC networking for a cluster
rosactl cluster-vpc create my-cluster --region us-east-1

# Create IAM resources (OIDC provider + roles) for a cluster
rosactl cluster-iam create my-cluster \
  --oidc-issuer-url https://oidc.example.com/my-cluster \
  --region us-east-1

# List cluster stacks
rosactl cluster-vpc list --region us-east-1
rosactl cluster-iam list --region us-east-1

# Delete cluster resources
rosactl cluster-vpc delete my-cluster --region us-east-1
rosactl cluster-iam delete my-cluster --region us-east-1

# Check version
rosactl version
```

## Prerequisites

- **Go 1.25+** (for building from source)
- **AWS credentials** configured via:
  - `~/.aws/credentials` file, or
  - Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`), or
  - IAM role (when running on EC2/ECS)
- **AWS IAM permissions**:
  - **CloudFormation**: `CreateStack`, `UpdateStack`, `DeleteStack`, `DescribeStacks`, `ListStacks`, `DescribeStackEvents`, `DescribeStackResources`, `ListStackResources`
  - **EC2** (for VPC): `CreateVpc`, `DeleteVpc`, `CreateSubnet`, `DeleteSubnet`, `CreateSecurityGroup`, `DeleteSecurityGroup`, `CreateNatGateway`, `DeleteNatGateway`, `CreateInternetGateway`, `DeleteInternetGateway`, `CreateRoute`, `DeleteRoute`, `CreateRouteTable`, `DeleteRouteTable`, `AuthorizeSecurityGroupEgress`, `AuthorizeSecurityGroupIngress`
  - **IAM** (for cluster roles): `CreateRole`, `DeleteRole`, `AttachRolePolicy`, `DetachRolePolicy`, `CreateInstanceProfile`, `DeleteInstanceProfile`, `AddRoleToInstanceProfile`, `RemoveRoleFromInstanceProfile`, `CreateOpenIDConnectProvider`, `DeleteOpenIDConnectProvider`, `GetOpenIDConnectProvider`, `ListOpenIDConnectProviders`
  - **Route53** (for VPC): `CreateHostedZone`, `DeleteHostedZone`

### Optional Tools

- **go-semver-release** - For semantic versioning (install: `go install github.com/s0ders/go-semver-release@latest`)
- **LocalStack** - For local testing with CloudFormation (see `test/localstack/README.md`)

## AWS Configuration

rosactl uses the AWS default credential chain:

```bash
# Option 1: AWS credentials file
cat ~/.aws/credentials
[default]
aws_access_key_id = YOUR_ACCESS_KEY
aws_secret_access_key = YOUR_SECRET_KEY
region = us-east-1

# Option 2: Environment variables
export AWS_ACCESS_KEY_ID=YOUR_ACCESS_KEY
export AWS_SECRET_ACCESS_KEY=YOUR_SECRET_KEY
export AWS_REGION=us-east-1

# Option 3: AWS profile
export AWS_PROFILE=your-profile-name
```

## Usage

### Cluster VPC Management

#### Create Cluster VPC

```bash
# Create VPC with default settings
rosactl cluster-vpc create my-cluster --region us-east-1

# Create with custom CIDR ranges
rosactl cluster-vpc create my-cluster \
  --region us-east-1 \
  --vpc-cidr 10.1.0.0/16 \
  --public-subnet-cidrs 10.1.101.0/24,10.1.102.0/24,10.1.103.0/24 \
  --private-subnet-cidrs 10.1.0.0/19,10.1.32.0/19,10.1.64.0/19

# Create with specific availability zones and per-AZ NAT gateways
rosactl cluster-vpc create my-cluster \
  --region us-east-1 \
  --availability-zones us-east-1a,us-east-1b,us-east-1c \
  --single-nat-gateway=false
```

**What this creates:**

- VPC with configurable CIDR block (default: 10.0.0.0/16)
- 3 public subnets across availability zones
- 3 private subnets across availability zones
- Internet Gateway
- NAT Gateway(s) - single (cost savings) or per-AZ (HA)
- Route tables and routes
- Security groups
- Route53 private hosted zone

#### List Cluster VPCs

```bash
# List all VPC stacks
rosactl cluster-vpc list --region us-east-1

# JSON output
rosactl cluster-vpc list --region us-east-1 --output json
```

#### Delete Cluster VPC

```bash
rosactl cluster-vpc delete my-cluster --region us-east-1
```

### Cluster IAM Management

#### Create Cluster IAM

```bash
# Create IAM resources (OIDC provider + roles)
rosactl cluster-iam create my-cluster \
  --oidc-issuer-url https://oidc.example.com/my-cluster \
  --region us-east-1
```

**What this creates:**

1. IAM OIDC Provider (with auto-fetched TLS thumbprint)
2. 7 control plane IAM roles:
   - Ingress Operator Role
   - Kube Controller Manager Role
   - EBS CSI Driver Operator Role
   - Image Registry Operator Role
   - Cloud Network Config Operator Role
   - Control Plane Operator Role
   - Node Pool Management Role
3. Worker node IAM role and instance profile

#### List Cluster IAM Stacks

```bash
# List all IAM stacks
rosactl cluster-iam list --region us-east-1

# JSON output
rosactl cluster-iam list --region us-east-1 --output json
```

#### Delete Cluster IAM

```bash
rosactl cluster-iam delete my-cluster --region us-east-1
```

### Optional: Lambda Bootstrap (Advanced)

For event-driven workflows and CI/CD integration, you can deploy rosactl as a Lambda function.

**Note:** Lambda is **optional** - all cluster management commands work directly without Lambda.

#### Deploy Lambda Bootstrap

```bash
# Build and push the container image to ECR
docker build -t <account>.dkr.ecr.us-east-1.amazonaws.com/rosactl:latest .
docker push <account>.dkr.ecr.us-east-1.amazonaws.com/rosactl:latest

# Deploy the Lambda function via CloudFormation
rosactl bootstrap create \
  --image-uri <account>.dkr.ecr.us-east-1.amazonaws.com/rosactl:latest \
  --function-name rosactl-bootstrap \
  --region us-east-1
```

#### Invoke the Lambda Handler

Once deployed, the Lambda function accepts JSON event payloads:

```json
{
  "action": "apply-cluster-vpc",
  "cluster_name": "my-cluster",
  "availability_zones": ["us-east-1a", "us-east-1b", "us-east-1c"],
  "single_nat_gateway": true
}
```

Supported `action` values:

- `apply-cluster-vpc` - Create or update VPC CloudFormation stack
- `delete-cluster-vpc` - Delete VPC stack
- `apply-cluster-iam` - Create or update IAM CloudFormation stack
- `delete-cluster-iam` - Delete IAM stack

```bash
# Invoke via AWS CLI
aws lambda invoke \
  --function-name rosactl-bootstrap \
  --payload '{"action":"apply-cluster-vpc","cluster_name":"my-cluster","availability_zones":["us-east-1a","us-east-1b","us-east-1c"]}' \
  --cli-binary-format raw-in-base64-out \
  response.json
```

### Version Management

```bash
# Check current version
rosactl version

# Check what next version would be (based on commits)
make release-dry-run

# Create a semantic version release
make release
```

See [docs/guides/VERSIONING.md](docs/guides/VERSIONING.md) for details on semantic versioning.

## Examples

### Complete Cluster Setup Workflow

```bash
# Step 1: Create VPC networking for the cluster
rosactl cluster-vpc create production-cluster --region us-east-1
# 🌐 Creating cluster VPC resources for: production-cluster
#    Region: us-east-1
#    VPC CIDR: 10.0.0.0/16
#    Single NAT Gateway: true
#
# 📄 Loading CloudFormation template...
# ☁️  Creating CloudFormation stack: rosa-production-cluster-vpc
#    This may take several minutes...
#
# ✅ Cluster VPC resources created successfully!
#    Stack ID: arn:aws:cloudformation:us-east-1:123456789012:stack/rosa-production-cluster-vpc/...
#
# Outputs:
#   VpcId: vpc-0abcd1234efgh5678
#   PublicSubnetIds: subnet-111,subnet-222,subnet-333
#   PrivateSubnetIds: subnet-444,subnet-555,subnet-666
#   PrivateHostedZoneId: Z1234567890ABC

# Step 2: Create IAM resources (OIDC provider and roles)
rosactl cluster-iam create production-cluster \
  --oidc-issuer-url https://oidc.example.com/production-cluster \
  --region us-east-1
# 🔐 Creating cluster IAM resources...
#    Cluster: production-cluster
#    OIDC Issuer: https://oidc.example.com/production-cluster
#    Region: us-east-1
#
# 🔍 Fetching TLS thumbprint from OIDC issuer...
#    Thumbprint: a1b2c3d4e5f67890a1b2c3d4e5f67890a1b2c3d4
#
# 📄 Loading CloudFormation template...
# ☁️  Creating CloudFormation stack: rosa-production-cluster-iam
#    This may take several minutes...
#
# ✅ Cluster IAM resources created successfully!
#    Stack ID: arn:aws:cloudformation:us-east-1:123456789012:stack/rosa-production-cluster-iam/...
#
# Created Resources:
#   OIDCProviderArn: arn:aws:iam::123456789012:oidc-provider/oidc.example.com/production-cluster
#   IngressOperatorRoleArn: arn:aws:iam::123456789012:role/production-cluster-ingress-operator
#   WorkerRoleArn: arn:aws:iam::123456789012:role/production-cluster-worker
#   WorkerInstanceProfileArn: arn:aws:iam::123456789012:instance-profile/production-cluster-worker

# Step 3: List created resources
rosactl cluster-vpc list --region us-east-1
rosactl cluster-iam list --region us-east-1

# Step 4: Cleanup when done
rosactl cluster-iam delete production-cluster --region us-east-1
rosactl cluster-vpc delete production-cluster --region us-east-1
```

### Custom VPC Configuration

```bash
# Create VPC with custom CIDR ranges and per-AZ NAT gateways for HA
rosactl cluster-vpc create my-cluster \
  --region us-west-2 \
  --vpc-cidr 10.1.0.0/16 \
  --public-subnet-cidrs 10.1.101.0/24,10.1.102.0/24,10.1.103.0/24 \
  --private-subnet-cidrs 10.1.0.0/19,10.1.32.0/19,10.1.64.0/19 \
  --availability-zones us-west-2a,us-west-2b,us-west-2c \
  --single-nat-gateway=false
```

## Development

### Build

```bash
make build
# Output: ./bin/rosactl
```

### Run Tests

```bash
# Set LocalStack Pro auth token (required for Lambda container tests)
export LOCALSTACK_AUTH_TOKEN=your-token-here

# LocalStack integration tests (CLI tests + Lambda handler invocation tests)
make test-localstack

# Install test dependencies
make test-deps
```

See [test/localstack/README.md](test/localstack/README.md) for LocalStack testing details.

### Clean

```bash
make clean
```

### Release a New Version

```bash
# Make changes with conventional commits
git commit -m "feat: add new feature"
git commit -m "fix: bug fix"

# Check what next version would be
make release-dry-run

# Create release tag
make release

# Push tag to GitHub
git push origin v0.2.0
```

## Project Structure

```
rosa-hyperfleet-cli/
├── cmd/rosactl/                     # Entry point
├── internal/
│   ├── commands/                    # CLI commands
│   │   ├── bootstrap/               # Lambda bootstrap deployment
│   │   ├── clustervpc/              # VPC management subcommands
│   │   ├── clusteriam/              # IAM management subcommands
│   │   ├── handler/                 # Lambda handler entrypoint command
│   │   └── version/                 # Version command
│   ├── services/                    # Shared business logic
│   │   ├── clustervpc/              # VPC service (CreateVPC, DeleteVPC)
│   │   └── clusteriam/              # IAM service (CreateIAM, DeleteIAM)
│   ├── aws/                         # AWS service clients
│   │   └── cloudformation/          # CloudFormation client and operations
│   ├── cloudformation/              # CloudFormation utilities
│   │   └── templates/               # Embedded CloudFormation templates
│   ├── crypto/                      # TLS thumbprint utilities
│   └── lambda/                      # Lambda event handler
├── test/
│   └── localstack/                  # LocalStack integration tests
├── docs/
│   ├── architecture/                # Architecture documentation
│   ├── guides/                      # User guides
│   └── specs/                       # Feature specifications
├── .semver.yaml                     # Semantic versioning config
├── Makefile                         # Build and test targets
├── go.mod
└── README.md
```

## Architecture

For detailed architecture documentation, see [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md).

**High-level overview:**

```
┌──────────────────────────────────────────────┐
│            rosactl CLI / Lambda Handler       │
│         (Cobra Framework)                    │
└───────────────┬──────────────────────────────┘
                │
    ┌───────────┼───────────────┐
    │           │               │
┌───▼────┐  ┌──▼────┐   ┌─────▼──────┐
│VPC Mgmt│  │IAM Mgmt│  │Lambda (opt)│
│Commands│  │Commands│  │Handler     │
└───┬────┘  └──┬─────┘  └─────┬──────┘
    │          │               │
    └──────────┼───────────────┘
               │
    ┌──────────▼──────────────┐
    │     Service Layer       │
    │  clustervpc / clusteriam│
    └──────────┬──────────────┘
               │
    ┌──────────▼──────────────┐
    │  CloudFormation Client  │
    │   (Embedded Templates)  │
    └──────────┬──────────────┘
               │
    ┌──────────▼──────────────┐
    │   AWS CloudFormation    │
    │  VPC | IAM | EC2 | R53  │
    └─────────────────────────┘
```

**Key Architectural Decisions:**

1. **Direct CloudFormation**: CLI commands directly create CloudFormation stacks (no Lambda required)
2. **Embedded Templates**: CloudFormation templates embedded in binary using go:embed for portability
3. **Service Layer**: Shared business logic in `internal/services/` used by both CLI commands and Lambda handler, avoiding duplication
4. **Optional Lambda**: Lambda bootstrap available for event-driven workflows, but not required for basic operations
5. **Typed Errors**: Custom error types for graceful handling of CloudFormation states (AlreadyExists, NoChanges, NotFound)

## CloudFormation Stack Naming

rosactl uses a consistent naming convention for CloudFormation stacks:

- **VPC stacks**: `rosa-{cluster-name}-vpc`
- **IAM stacks**: `rosa-{cluster-name}-iam`

All stacks are tagged with:

- `Cluster`: cluster name
- `ManagedBy`: rosactl
- `red-hat-managed`: true

## Security Considerations

### OIDC Thumbprint Auto-Fetch

When creating IAM resources with `cluster-iam create`, rosactl automatically fetches the TLS thumbprint from the OIDC issuer URL. This requires:

- The OIDC issuer URL to be publicly accessible over HTTPS
- Valid TLS certificate on the OIDC endpoint

### IAM Roles Created

The `cluster-iam create` command creates the following IAM resources via CloudFormation:

**Control Plane Roles** (7):

- Ingress Operator Role
- Kube Controller Manager Role
- EBS CSI Driver Operator Role
- Image Registry Operator Role
- Cloud Network Config Operator Role
- Control Plane Operator Role
- Node Pool Management Role

**Worker Node Resources**:

- Worker IAM Role
- Worker Instance Profile

All roles use OIDC federation for authentication with minimal required permissions.

### VPC Resources Created

The `cluster-vpc create` command creates isolated networking resources:

- Dedicated VPC with configurable CIDR
- Public and private subnets across 3 availability zones
- NAT Gateway(s) for outbound internet access from private subnets
- Route53 private hosted zone for internal DNS

## Lambda-Specific Information (Optional Feature)

### Lambda IAM Execution Roles

When using the optional Lambda bootstrap feature, rosactl automatically creates IAM execution roles:

1. **`rosactl-lambda-execution-role`** - Basic Lambda execution role
   - Policy: `AWSLambdaBasicExecutionRole` (CloudWatch Logs)

2. **`rosactl-lambda-oidc-execution-role`** - OIDC Lambda execution role
   - Policy: `AWSLambdaBasicExecutionRole` (CloudWatch Logs)
   - Inline policy: S3 bucket management (`oidc-issuer-*` buckets)
   - Inline policy: IAM OIDC provider management

**Note:** These roles are NOT deleted when Lambda functions are removed.

### Lambda OIDC RSA Private Keys

When creating OIDC Lambdas (`--handler oidc`), the RSA private key is saved to:

```
/tmp/oidc-private-key-{KEY_ID}.pem
```

**Security best practices:**

- File permissions are set to `0600` (owner read/write only)
- Move the key to a secure location (e.g., AWS Secrets Manager) for production use
- Delete from `/tmp` when no longer needed
- **Never commit private keys to version control**

## Troubleshooting

### Common Issues

**"Stack already exists" (cluster-vpc or cluster-iam create)**

- The command automatically attempts to update the existing stack
- Check the stack status in CloudFormation console
- If stuck in a failed state, delete and recreate:
  ```bash
  rosactl cluster-vpc delete my-cluster --region us-east-1
  rosactl cluster-vpc create my-cluster --region us-east-1
  ```

**"Failed to fetch TLS thumbprint" (cluster-iam create)**

- Ensure the OIDC issuer URL is publicly accessible over HTTPS
- Verify the TLS certificate is valid
- Check network connectivity to the OIDC endpoint

**"Insufficient permissions" (CloudFormation errors)**

- Ensure your AWS credentials have the required permissions listed in Prerequisites
- Check CloudFormation stack events for specific permission errors:
  ```bash
  aws cloudformation describe-stack-events --stack-name rosa-my-cluster-vpc
  ```

**"NAT Gateway creation timeout" (LocalStack testing)**

- This is expected in LocalStack as NAT Gateway support is limited
- Tests accept both CREATE_COMPLETE and CREATE_FAILED status for LocalStack
- Real AWS environments should succeed

**"Lambda container execution fails" (LocalStack testing)**

- Lambda container execution requires LocalStack Pro
- Set `LOCALSTACK_AUTH_TOKEN=your-token-here` before starting LocalStack
- Or create a `.env` file in the project root with `LOCALSTACK_AUTH_TOKEN=your-token-here`

**AWS Configuration**

```bash
# Set AWS profile
export AWS_PROFILE=your-profile-name

# Or use environment variables
export AWS_ACCESS_KEY_ID=your-key
export AWS_SECRET_ACCESS_KEY=your-secret
export AWS_REGION=us-east-1
```

**"go-semver-release not found"**

```bash
go install github.com/s0ders/go-semver-release@latest
```

## Documentation

- [Architecture](docs/architecture/ARCHITECTURE.md) - System architecture and design decisions
- [Versioning Guide](docs/guides/VERSIONING.md) - Semantic versioning with conventional commits
- [Development Guide](docs/guides/DEVELOPMENT.md) - Development setup and guidelines
- [LocalStack Testing Guide](test/localstack/README.md) - Running integration tests with LocalStack

## Contributing

Contributions are welcome! Please follow the conventional commit format for commit messages:

```bash
# Features (minor version bump)
git commit -m "feat: add custom domain support"

# Bug fixes (patch version bump)
git commit -m "fix: handle timeout errors"

# Other changes (patch version bump)
git commit -m "docs: update README"
git commit -m "chore: update dependencies"
git commit -m "refactor: simplify OIDC creation"
```

See [docs/guides/VERSIONING.md](docs/guides/VERSIONING.md) for details.

## License

Apache License 2.0

## Acknowledgments

Built with:

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [AWS SDK for Go v2](https://github.com/aws/aws-sdk-go-v2) - AWS integration
- [Ginkgo](https://github.com/onsi/ginkgo) - Testing framework
- [go-semver-release](https://github.com/s0ders/go-semver-release) - Semantic versioning
