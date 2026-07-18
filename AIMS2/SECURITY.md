# Security Policy

## Reporting a vulnerability

Please do not open a public issue for a suspected vulnerability, exposed credential, private inventory data, or sensitive conversation log. Use GitHub's private vulnerability reporting feature under the repository's **Security** tab. Include reproduction steps, affected versions or commits, and the potential impact when possible.

## Sensitive data

AIMS2 can process confidential inventory, pricing, cost, note, vendor, and conversation data. Deployers are responsible for:

- keeping credentials and private endpoints outside source control;
- restricting access to inventory databases and model services;
- disabling or protecting conversation logs;
- reviewing model and tool payloads before sending them to third-party services;
- enforcing authentication, authorization, and least-privilege database access; and
- retaining explicit approval for state-changing operations.

The repository must contain synthetic fixtures only. Real inventory databases, spreadsheet exports, conversation logs, and env files are intentionally ignored.
