# Reference Projects

This document lists GitHub repositories that serve as guardrails and guidelines for the development of regional-cli.

## Reference Repositories

### [hypershift](https://github.com/openshift/hypershift)

**Purpose:** Implements the hypershift compoent

**Key Takeaways:**

- the rosa regional hcp service deployes hypershift clusters

**Relevant Areas:**

- hypershift cli can create oidc provider in the customer account, some of our cli functions can draw from this repository

### [rosa-hyperfleet-api](https://github.com/openshift-online/rosa-hyperfleet-api)

**Purpose:** Implements the backend passthrough api that this cli will connect to

**Key Takeaways:**

- What patterns or approaches to follow
- What to avoid
- Specific implementation details to reference

**Relevant Areas:**

- backend that this cli will connect too
- implements authz based on cedar policies

### [rosa](https://github.com/openshift/rosa)

**Purpose:** Implements the current rosa cli tool customers use to interact with the rosa hcp service.

**Key Takeaways:**

- CLI structure and command organization
- Authentication patterns
- API client patterns

**Relevant Areas:**

- Overall CLI architecture
- Command structure
- API integration
