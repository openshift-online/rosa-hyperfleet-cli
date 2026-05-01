# Security Audit — rosa-regional-platform-cli

> **Audit Date:** 2026-05-01  
> **Auditor:** Automated adversarial security agent  
> **Scope:** Full repository static analysis — Go source, Dockerfile, docker-compose, CI configuration

---

## Summary

This repository implements the `rosactl` CLI tool for managing ROSA Regional Platform clusters. It creates VPC networking, IAM roles, OIDC providers, and CloudFormation stacks on AWS. The audit identified **0 CRITICAL**, **0 HIGH**, **4 MEDIUM**, and **5 LOW** findings. The codebase demonstrates generally sound security practices — no hardcoded secrets, proper SigV4 signing, explicit TLS, config file permissions of `0600`. The most significant issues are cluster configuration files written world-readable, a mandatory SHA-1 usage (AWS-required but worth explicit documentation), a privileged LocalStack container used in tests, and TLS verification disabled in test code that could be copied into production use.

---

## Findings

### MEDIUM

---

**[MEDIUM] Cluster Configuration Output Files Written with `0644` Permissions (World-Readable)**

- **File:** `internal/commands/cluster/create.go` (lines 197, 243)
- **Category:** Application — Information Disclosure / File Permissions
- **Issue:** Generated cluster configuration files are written with `os.WriteFile(..., 0644)`. These files contain CloudFormation outputs including VPC IDs, subnet IDs, IAM role ARNs, and OIDC provider URLs — infrastructure topology data that should be considered sensitive.

  ```go
  if err := os.WriteFile(opts.outputFile, jsonBytes, 0644); err != nil {
  ```

  On a shared or multi-user system (e.g., a CI runner, a shared developer workstation, or a container with multiple processes), any user or process can read these files.

- **Attack Vector:** A local attacker or a compromised co-located process reads the cluster configuration file to enumerate the full network topology, IAM role ARNs, and OIDC endpoint. This information directly enables more targeted attacks against the deployed infrastructure (assuming specific role ARNs, knowing which subnets to target, etc.).

- **Impact:** Infrastructure topology disclosure. An attacker who reads these files can significantly reduce the reconnaissance cost for subsequent attacks against the cluster.

- **Recommendation:** Change to `0600`:
  ```go
  if err := os.WriteFile(opts.outputFile, jsonBytes, 0600); err != nil {
  ```

---

**[MEDIUM] SHA-1 Used for OIDC Certificate Thumbprint — AWS-Mandated but Undocumented**

- **File:** `internal/crypto/thumbprint.go` (line 62)
- **Category:** Application — Cryptography
- **Issue:** The OIDC provider thumbprint is computed using SHA-1:

  ```go
  hash := sha1.Sum(rootCert.Raw)
  ```

  SHA-1 is cryptographically deprecated and collision attacks exist. While this is required by the AWS IAM OIDC API, there is no code comment documenting this constraint or linking to the AWS specification. Future maintainers may attempt to "fix" this to SHA-256, breaking OIDC registration. Worse, a future change to wrap this function for other purposes could inadvertently use SHA-1 where it is insecure.

- **Attack Vector:** SHA-1 collision attacks (computationally expensive, but not impossible for nation-state actors) could allow an attacker to craft a certificate with a matching thumbprint that AWS IAM trusts as the legitimate OIDC provider, enabling JWT forgery for cluster service accounts.

- **Recommendation:** Add an explicit comment citing the AWS requirement. Monitor AWS IAM documentation for migration to SHA-256 thumbprints. Ensure this function is only ever called for OIDC thumbprint computation and not reused for other purposes.

---

**[MEDIUM] LocalStack Test Container Runs as Root with `privileged: true`**

- **File:** `docker-compose.localstack.yaml` (line 55)
- **Category:** Container Security — Privilege Escalation
- **Issue:** The LocalStack container used in integration tests runs with `privileged: true` and `user: "0"` (root), which is required for Docker-in-Docker Lambda execution:

  ```yaml
  user: "0"
  privileged: true
  ```

  If this configuration is used in a CI/CD environment without proper isolation, a compromised LocalStack image or exploited Lambda test could escape the container and access the Docker daemon, potentially reading secrets from other containers.

- **Attack Vector:** Supply chain attack on the `localstack/localstack` image pushes a malicious version. When CI runs integration tests, the privileged LocalStack container breaks out to the host, reads secrets mounted to other CI containers (e.g., AWS credentials for the real account), and exfiltrates them.

- **Impact:** CI/CD environment compromise, potential credential exfiltration for real AWS accounts.

- **Recommendation:** Add a prominent warning in docker-compose.localstack.yaml that this is for local development only and must not be used in CI with access to production credentials. Evaluate whether `--privileged` can be replaced with a more targeted capability (`SYS_ADMIN`) or whether Lambda execution tests can be run differently.

---

**[MEDIUM] TLS Verification Disabled in Container Image Push Test Code**

- **File:** `test/localstack/lambda_test.go` (line 378)
- **Category:** Application — Insecure TLS Configuration
- **Issue:** The LocalStack integration test explicitly disables TLS verification when pushing container images:

  ```go
  "--tls-verify=false",
  ```

  While this is acceptable for a local-only LocalStack setup, it establishes a copy-paste pattern that could be reused in production tooling. The flag appears without a comment explaining it's test-only.

- **Attack Vector:** A developer copies this code pattern when writing image push logic for a real registry, inadvertently disabling TLS certificate verification in production. A MITM attacker between the CI runner and a real registry substitutes a malicious image, bypassing certificate validation.

- **Recommendation:** Add an explicit comment:
  ```go
  // SECURITY: TLS disabled ONLY for local LocalStack testing. NEVER use in production.
  "--tls-verify=false",
  ```
  Consider extracting this into a helper with a name that makes the unsafety obvious (`pushImageToLocalTestRegistry`).

---

### LOW

---

**[LOW] Configuration Directory Created with `0755` Permissions (World-Listable)**

- **File:** `internal/config/config.go` (line 44)
- **Category:** Application — File Permissions
- **Issue:** The `.rosactl` configuration directory is created with `0755`, making it world-listable. The config file inside uses `0600` (good), but the directory listing reveals that the user has configured the CLI and which files exist in the config directory.

  ```go
  if err := os.MkdirAll(configDirPath, 0755); err != nil {
  ```

- **Recommendation:** Use `0700` for the configuration directory to apply defense in depth:
  ```go
  if err := os.MkdirAll(configDirPath, 0700); err != nil {
  ```

---

**[LOW] HTTP Clients Use Default Configuration Without Explicit TLS Version Floor**

- **File:** `internal/services/cluster/service.go` (line 183), `internal/commands/cluster/list.go` (line 134)
- **Category:** Application — TLS Configuration
- **Issue:** HTTP clients are created without explicit TLS configuration:

  ```go
  client := &http.Client{
      Timeout: 30 * time.Second,
  }
  ```

  Go's defaults are secure (TLS 1.2+, certificate verification enabled), but there is no explicit floor on minimum TLS version or documentation that this is intentional. A future change that adds a custom `Transport` might silently downgrade these settings.

- **Recommendation:** Explicitly document TLS behavior. If a custom transport is ever added, require a security review before changing TLS parameters.

---

**[LOW] API Error Responses Include Full HTTP Response Body**

- **File:** `internal/services/cluster/service.go` (line 201)
- **Category:** Application — Information Disclosure
- **Issue:**
  ```go
  return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
  ```
  If the platform API returns stack traces or internal error details, these propagate directly to CLI output and potentially to logs or error reporting systems.

- **Recommendation:** Truncate or sanitize API error bodies in non-verbose mode. In verbose mode, log the full error body locally but don't forward it to external systems.

---

**[LOW] Lambda Handler Logs Full Event Object**

- **File:** `internal/lambda/handler.go` (line 38)
- **Category:** Application — Information Disclosure
- **Issue:**
  ```go
  fmt.Printf("Received event: %+v\n", event)
  ```
  If the event structure is extended to include credentials, tokens, or sensitive configuration, they will be logged to CloudWatch without redaction.

- **Recommendation:** Log only specific non-sensitive fields (action, cluster name). Add a review gate to the event struct to catch new fields that shouldn't be logged.

---

**[LOW] AWS Region Environment Variable Not Validated Against Known Region List**

- **File:** `internal/aws/session.go` (lines 14–15)
- **Category:** Application — Input Validation
- **Issue:** `ROSACTL_REGION` is accepted and forwarded to the AWS SDK without format validation. While the SDK will reject invalid regions, early validation provides clearer error messages and prevents user confusion.

- **Recommendation:** Validate against a known list of AWS regions or at minimum against the format pattern `[a-z]{2}-[a-z]+-[0-9]`.

