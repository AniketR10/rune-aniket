# Security Policy

## Supported Versions

Security fixes are applied to the latest release. Older releases do not
receive backported fixes.

## Reporting a Vulnerability

Please do not report security vulnerabilities through public GitHub
issues, discussions, or pull requests.

Report vulnerabilities by email to **security@unstable.build**, or through
[GitHub private vulnerability
reporting](https://github.com/unstablebuild/rune/security/advisories/new).

Include as much of the following as you can:

- The type of issue and its impact
- Affected version(s) and platform(s)
- Step-by-step instructions or a proof of concept to reproduce the issue
- Any relevant configuration (extensions, agent providers, workspace setup)

You should receive an acknowledgment within a few days. We follow
coordinated disclosure: please give us a reasonable window (90 days) to
ship a fix before disclosing publicly. We will credit reporters in the
release notes unless you prefer otherwise.

## Scope notes

Rune Agent executes model-driven tool calls (file edits, shell commands)
under user-configured permission modes. Reports of sandbox or permission
bypasses — e.g. prompt-injection paths that escalate beyond the configured
permission mode, or tool-call escapes from the sandbox — are considered
security vulnerabilities and are in scope.
