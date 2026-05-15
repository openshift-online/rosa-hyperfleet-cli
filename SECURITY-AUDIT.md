# Security Audit — rosa-regional-platform-cli

**Audit Date:** 2026-05-15  
**Auditor:** security-audit-agent  
**Severity Labels:** CRITICAL / HIGH / MEDIUM / LOW

---

## Finding 1 — HIGH: Lambda IAM Role Has Unrestricted Resource Scope for IAM Operations (Privilege Escalation Path)

**File:** `internal/cloudformation/templates/lambda-bootstrap.yaml:88-102`

```yaml
- Sid: IAMResourceManagement
  Effect: Allow
  Action:
    - iam:CreateRole
    - iam:DeleteRole
    - iam:GetRole
    - iam:TagRole
    - iam:UntagRole
    - iam:AttachRolePolicy
    - iam:DetachRolePolicy
    - iam:PutRolePolicy
    - iam:DeleteRolePolicy
    - iam:GetRolePolicy
    - iam:CreateInstanceProfile
    - iam:DeleteInstanceProfile
    - iam:GetInstanceProfile
    - iam:AddRoleToInstanceProfile
    - iam:RemoveRoleFromInstanceProfile
    - iam:TagInstanceProfile
    - iam:UntagInstanceProfile
  Resource: '*'
```

**Risk:**  
The Lambda execution role can create IAM roles, attach **any** AWS-managed or customer-managed policy to them, and inline arbitrary policies — all on `Resource: *` (any IAM resource). This is a **privilege escalation path**: a compromised Lambda function can create a new IAM role, attach `AdministratorAccess` to it, and assume that role, gaining full control of the AWS account.

The intended use is to create cluster-specific IAM roles for ROSA components. However, there is no IAM permission boundary, path restriction, or `iam:PermissionsBoundary` condition to limit what roles can be created or what policies can be attached.

**Attack Vectors:**
1. **Lambda code injection:** If the Lambda function has an event-processing bug (e.g., unsafe deserialization of CloudFormation event parameters, path traversal in template loading), an attacker could inject arbitrary IAM operations.
2. **Compromised container image:** The Lambda runs from a container image (`ContainerImageURI`). If the image is compromised (especially since the default hints at `:latest` style URIs), the malicious code runs with these IAM permissions.
3. **CloudFormation parameter injection:** A malicious CloudFormation stack parameter (e.g., `ClusterName` with special characters) could potentially influence IAM operations if the Lambda constructs role names or policy documents from parameters without sanitization.

**What to Mitigate:**
1. **Add IAM Permission Boundaries:** Create a permission boundary policy and require it on all roles created by the Lambda: `iam:CreateRole` with condition `iam:PermissionsBoundary: arn:aws:iam::${AccountId}:policy/rosa-cluster-boundary`.
2. **Restrict resource scope:** Limit `Resource` to a path prefix: `arn:aws:iam::${AccountId}:role/rosa-*` and `arn:aws:iam::${AccountId}:instance-profile/rosa-*`.
3. **Restrict `AttachRolePolicy`:** Add a condition to allow only specific AWS-managed policies (ROSAIngressOperatorPolicy, etc.) to be attached: `iam:PolicyARN: arn:aws:iam::aws:policy/service-role/ROSA*`.

---

## Finding 2 — HIGH: Lambda Bootstrap IAM Role Has Unrestricted PassRole and VPC/Route53 Resource Scope

**File:** `internal/cloudformation/templates/lambda-bootstrap.yaml:52,96,137,152`

```yaml
- Sid: PassRoleForCloudFormation
  Effect: Allow
  Action: iam:PassRole
  Resource: '*'    # line 52
  Condition:
    StringEquals:
      iam:PassedToService: cloudformation.amazonaws.com

- Sid: VPCResourceManagement
  # ... 
  Resource: '*'    # line 96 — all EC2 resources

- Sid: Route53ResourceManagement
  # ...
  Resource: '*'    # line 137 — all Route53 resources
```

**Risk:**  

**PassRole `Resource: *`:** Although scoped to CloudFormation via the condition, `iam:PassRole` on `Resource: *` means the Lambda can pass **any** IAM role in the account to CloudFormation stacks. If an existing over-privileged role exists (e.g., an admin role), the Lambda (or a compromised CloudFormation stack it deploys) could operate with that role's permissions.

**VPCResourceManagement `Resource: *`:** This permits operations like `ec2:CreateSecurityGroup`, `ec2:AuthorizeSecurityGroupIngress/Egress`, and `ec2:CreateVpc` on all EC2 resources in the account. A compromised Lambda could modify security groups for unrelated VPCs, affecting other tenants or services.

**Route53ResourceManagement `Resource: *`:** Allows creating/deleting hosted zones and associating/disassociating VPCs from hosted zones for **all** Route53 resources. A compromised Lambda could disrupt DNS for other services.

**What to Mitigate:**
- Restrict `iam:PassRole` `Resource` to the specific cluster role ARN patterns: `arn:aws:iam::${AccountId}:role/rosa-*`
- Restrict EC2 actions using `ec2:ResourceTag/ManagedBy: rosactl` resource conditions
- Restrict Route53 actions to hosted zone ARNs tagged for ROSA clusters

---

## Finding 3 — MEDIUM: CLI Config Directory Created with World-Readable Permissions

**File:** `internal/config/config.go:44`

```go
func ensureConfigDir() error {
    // ...
    if err := os.MkdirAll(configDirPath, 0755); err != nil {
```

**Risk:**  
The `~/.rosactl/` configuration directory is created with `0755` permissions, making it **world-readable** (any user on the system can list and read files in it). The config file itself is saved with `0600` (line 87: `os.WriteFile(configPath, data, 0600)`), which is correct. However, the directory being `0755` means:
- Other users can see that the directory exists and list its contents (the filename `config.json` is visible)
- On some systems, directory read + execute permissions allow other users to access files if they know the filename (combined with file permissions, `0600` prevents actual reads)

The primary risk is **metadata leakage**: the directory listing reveals that a user has configured `rosactl`, which platform API URL they're connecting to (if the config were ever more permissive), and that credentials exist.

**What to Mitigate:**  
Change `os.MkdirAll(configDirPath, 0755)` to `os.MkdirAll(configDirPath, 0700)` to restrict the config directory to the owner only. This follows the principle of least privilege and matches the pattern used by tools like `~/.aws/` and `~/.ssh/`.

---

## Finding 4 — MEDIUM: Unpinned Container Image URI Accepted Without Digest Validation

**File:** `internal/cloudformation/templates/lambda-bootstrap.yaml:2-8`

```yaml
Parameters:
  ContainerImageURI:
    Type: String
    Description: 'ECR container image URI (e.g., 123456789012.dkr.ecr.us-east-1.amazonaws.com/rosa-regional-platform-cli:latest)'
```

**Risk:**  
The CloudFormation parameter description uses `:latest` as an example and does not enforce a digest-pinned format. Users deploying this template may use `:latest` or a mutable tag, exposing the Lambda to the supply chain risks described in the platform repo audit. The Lambda runs with the IAM permissions described in Findings 1 and 2 above — a compromised image gets full IAM escalation capabilities.

**Attack Vector:**  
A developer deploys with `rosa-regional-platform-cli:latest`. The container image registry is compromised and a malicious image is pushed with the same tag. On next Lambda cold start or explicit update, the malicious code runs with the Lambda's IAM permissions, performing privilege escalation as described in Finding 1.

**What to Mitigate:**
- Add an `AllowedPattern` constraint to the `ContainerImageURI` parameter that requires a SHA256 digest: `^.*@sha256:[a-f0-9]{64}$`
- Or add validation in the Lambda deployment tooling to reject mutable tags
- Update the description to explicitly warn against using `:latest` or mutable tags

---

## Finding 5 — LOW: `MustGetPlatformAPIURL` Uses `os.Exit(1)` Instead of Returning an Error

**File:** `internal/config/config.go:101-107`

```go
func MustGetPlatformAPIURL() string {
    url, err := GetPlatformAPIURL()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    return url
}
```

**Risk:**  
Using `os.Exit(1)` bypasses Go's deferred function cleanup, which can lead to resource leaks (open file handles, unclosed connections) in callers. More critically from a security perspective, it bypasses any cleanup logic that might clear sensitive data from memory (credentials, tokens) or close authenticated sessions.

**What to Mitigate:**  
Return an `error` from `MustGetPlatformAPIURL` and let callers handle it with proper cleanup. The Cobra framework already has a mechanism to exit with an error code via `RunE`, so callers can propagate errors cleanly without `os.Exit`.
