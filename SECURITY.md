# Security Policy

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| latest  | ✅ Security patches |
| < 1.0   | ⚠️ Best effort      |

Security patches are applied to the latest minor release. Pre-1.0 releases receive best-effort fixes for critical vulnerabilities.

## Reporting a Vulnerability

**Please do NOT open a public GitHub issue for security vulnerabilities.**

Instead, report vulnerabilities via email:

📧 **security@kruda.dev**

Include the following in your report:

- Description of the vulnerability
- Steps to reproduce
- Affected version(s)
- Impact assessment (if known)
- Any suggested fix or mitigation

## Response Timeline

| Stage                  | Timeline                          |
|------------------------|-----------------------------------|
| Acknowledgment         | Within **48 hours** of receipt    |
| Vulnerability assessment | Within **7 days** of acknowledgment |
| Coordinated disclosure | **90 days** from initial report   |

We follow coordinated disclosure. If a fix is available before the 90-day window, we will coordinate with the reporter on disclosure timing. If no fix is available after 90 days, the reporter may disclose at their discretion.

## Scope

The following components are in scope for security reports:

- **Framework core** — `github.com/go-kruda/kruda` (router, context, config, error handling, lifecycle)
- **Built-in middleware** — `middleware/` (logger, recovery, CORS, request ID, timeout)
- **Wing transport** — `transport/wing/` (HTTP parser, epoll engine, connection management — Linux only)
- **CLI tool** — `cmd/kruda/` (project scaffolding, dev server)

### Out of Scope

- Vulnerabilities in user application code
- Third-party dependencies (report to the dependency maintainer directly)
- `contrib/` modules (each has its own security policy)
- Denial of service via legitimate high traffic (infrastructure concern)

## Recognition

We credit reporters in our security advisories and CHANGELOG unless anonymity is requested.
