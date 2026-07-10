# Email notifications and reports

PatchBase can send email notifications about vulnerability status and advisory updates. This is optional — the dashboard works fine without it.

## Configuring SMTP

Email settings are configured from the dashboard under **Settings**. You'll need:

- **SMTP host** and **port** (e.g., `smtp.gmail.com:587`)
- **Username** and **password** (or app-specific password)
- **From address** — the sender for outgoing emails
- **Use TLS** — enable for port 587 (STARTTLS) or 465 (implicit TLS)

You can test the configuration with the **Send test email** button before saving.

## Mailpit for development

The included Docker Compose file runs [Mailpit](https://github.com/axllent/mailpit) — a local SMTP catcher with a web UI. It's available at `http://localhost:8025` and catches all email on port 1025 without actually sending anything.

For local development, configure SMTP as:

- Host: `localhost`
- Port: `1025`
- No authentication
- No TLS

## Sending reports

From the dashboard, you can trigger a vulnerability report to be sent to the configured email address. The report includes:

- A summary of hosts needing attention
- Vulnerability counts by severity
- Recent advisories affecting your fleet

This is a manual action for now. Future versions will add scheduled report delivery.