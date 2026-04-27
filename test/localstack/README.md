# LocalStack Integration Tests

This directory contains integration tests that run `rosactl` commands against LocalStack.

## Overview

These tests verify that the CLI correctly creates AWS resources (VPC, IAM, CloudFormation stacks) by running against a local LocalStack instance instead of real AWS.

## Prerequisites

- **Docker or Podman** and Docker Compose
  - Podman users (Fedora/RHEL): The compose file automatically uses your Podman socket at `/run/user/$(id -u)/podman/podman.sock`
  - Docker users: Set `export DOCKER_SOCK=/var/run/docker.sock` before running
- **LocalStack Pro** - Lambda container execution tests require LocalStack Pro
  - Set `export LOCALSTACK_AUTH_TOKEN=your-token-here` before running, or create a `.env` file containing `LOCALSTACK_AUTH_TOKEN=your-token-here`
  - A free LocalStack Pro trial or paid subscription provides the auth token
- Go 1.25+
- Ginkgo CLI (install if not present):
  ```bash
  go install github.com/onsi/ginkgo/v2/ginkgo@latest
  ```
- AWS CLI v2 (optional, for manual inspection)

## Running Tests

### Quick Start

```bash
# Set LocalStack Pro auth token (required for Lambda container tests)
export LOCALSTACK_AUTH_TOKEN=your-token-here

# From the project root
./test/localstack/run-localstack-tests.sh
```

This script will:
1. Start LocalStack Pro via docker-compose
2. Build the `rosactl` binary
3. Run all Ginkgo tests against LocalStack (CLI tests and Lambda handler tests)
4. Optionally stop LocalStack when done

### Manual Execution

```bash
# 0. (Docker users only) Set Docker socket location
export DOCKER_SOCK=/var/run/docker.sock

# 1. Start LocalStack
docker-compose -f docker-compose.localstack.yaml up -d

# 2. Build binary
go build -o rosactl ./cmd/rosactl

# 3. Set environment variables
export LOCALSTACK_ENDPOINT="http://localhost:4566"
export AWS_ACCESS_KEY_ID="test"
export AWS_SECRET_ACCESS_KEY="test"
export AWS_REGION="us-east-1"

# 4. Run tests
cd test/localstack
ginkgo -v

# 5. Cleanup
docker-compose -f docker-compose.localstack.yaml down -v
```

## What's Tested

### VPC Management via CLI (`cluster-vpc`)
- CloudFormation template validation
- VPC resource creation
- Subnet creation across availability zones
- Security group creation
- Route53 private hosted zone creation

### IAM Management via CLI (`cluster-iam`)
- CloudFormation template validation
- IAM OIDC provider creation
- Control plane IAM roles (7 roles)
- Worker node IAM role and instance profile

### CloudFormation Operations
- Stack creation
- Stack listing
- Stack status checking
- Stack deletion

### Lambda Handler Invocations (`lambda_test.go`)
- Builds the `rosactl` container image and pushes it to LocalStack ECR
- Deploys the Lambda function via `rosactl bootstrap create`
- Invokes Lambda with `apply-cluster-vpc` event and verifies VPC stack creation
- Invokes Lambda with `apply-cluster-iam` event and verifies IAM stack creation
- Invokes Lambda with `delete-cluster-vpc` event and verifies VPC stack deletion
- Invokes Lambda with `delete-cluster-iam` event and verifies IAM stack deletion

## Test Structure

```
test/localstack/
├── README.md                      # This file
├── run-localstack-tests.sh        # Test runner script
├── localstack_suite_test.go       # Ginkgo suite setup
├── localstack_test.go             # CLI integration tests
└── lambda_test.go                 # Lambda handler invocation tests
```

## LocalStack Configuration

The `docker-compose.localstack.yaml` file configures LocalStack Pro with:
- CloudFormation
- IAM
- EC2
- Route53
- Lambda (container image execution via LocalStack Pro)
- ECR (container image storage and retrieval)
- S3
- CloudWatch Logs

**Note**: Lambda container execution requires LocalStack Pro. Set `LOCALSTACK_AUTH_TOKEN` before starting LocalStack.

## Current Limitations

1. **LocalStack Pro Required**: Lambda container execution tests require a LocalStack Pro subscription. Set `LOCALSTACK_AUTH_TOKEN` before running the full test suite.

2. **NAT Gateway Limitations**: LocalStack's NAT Gateway support is limited. Tests that create VPC stacks accept both `CREATE_COMPLETE` and `CREATE_FAILED` stack status, since NAT Gateway creation may fail in LocalStack.

3. **Resource Validation**: Tests verify that resources are created but don't deeply validate all properties (e.g., IAM policy attachments, security group rules).

## Future Enhancements

- [ ] Validate CloudFormation stack outputs in detail
- [ ] Test stack update operations
- [ ] Test error handling and rollback scenarios
- [ ] Add integration with CI/CD (Prow)

## Debugging

### View LocalStack logs
```bash
docker-compose -f docker-compose.localstack.yaml logs -f
```

### Check LocalStack health
```bash
curl http://localhost:4566/_localstack/health
```

### List resources in LocalStack
```bash
# CloudFormation stacks
aws cloudformation list-stacks --endpoint-url http://localhost:4566

# VPCs
aws ec2 describe-vpcs --endpoint-url http://localhost:4566

# IAM roles
aws iam list-roles --endpoint-url http://localhost:4566
```

### Keep LocalStack running after tests
Edit `run-localstack-tests.sh` and comment out the cleanup section, or answer 'N' when prompted to stop LocalStack.

## Troubleshooting

**Problem**: LocalStack doesn't start
- **Podman users**: Make sure Podman socket is running: `systemctl --user status podman.socket`
  - Start if needed: `systemctl --user start podman.socket`
- **Docker users**: Set `export DOCKER_SOCK=/var/run/docker.sock` before starting
- Check Docker/Podman is running: `docker ps` or `podman ps`
- Check logs: `docker-compose -f docker-compose.localstack.yaml logs`

**Problem**: Tests fail with connection refused
- Verify LocalStack is healthy: `curl http://localhost:4566/_localstack/health`
- Check `LOCALSTACK_ENDPOINT` environment variable is set correctly

**Problem**: CloudFormation stack creation hangs
- LocalStack may not support all CloudFormation resource types
- Check LocalStack logs for unsupported features
- Consider using LocalStack Pro for advanced features
