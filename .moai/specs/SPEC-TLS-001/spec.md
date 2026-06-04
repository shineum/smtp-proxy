---
id: SPEC-TLS-001
version: "1.0.0"
status: approved
created: "2026-06-03"
updated: "2026-06-03"
author: sungwon
priority: P1
lifecycle: spec-anchored
domains:
  - infrastructure
  - security
  - tls
tags:
  - tls
  - certificates
  - aws-secrets-manager
  - hot-reload
  - sni
  - certbot
related:
  - SPEC-INFRA-001
---

# SPEC-TLS-001: AWS Secrets Manager TLS Certificate Provider

## HISTORY

### Version 1.0.0 (2026-06-03)
- Initial specification for loading TLS certificates from AWS Secrets Manager
- Periodic hot-reload without server restart
- Graceful fallback to in-memory self-signed certificate
- SNI-based multi-domain certificate selection

## OVERVIEW

The SMTP server currently loads its TLS certificate once at startup, either
from files (`SMTP_PROXY_TLS_CERT_FILE`/`KEY_FILE`) or an auto-generated
self-signed certificate. When deployed to AWS, real certificates are issued by
an external certbot process and published to AWS Secrets Manager. This SPEC
adds a certificate provider that periodically reads certificates from Secrets
Manager and hot-reloads them into the running server, while preserving the
existing self-signed behavior when Secrets Manager is not configured or
unavailable.

Design goal: minimal change to the existing structure. The feature is opt-in via
a single required environment variable; without it, behavior is identical to
today.

## SCOPE

### In Scope

1. Load certificates from AWS Secrets Manager by secret ID (env var).
2. Periodic reload at a configurable interval (hours, default 168 = 7 days).
3. Hot-reload via `tls.Config.GetCertificate` callback (no restart).
4. SNI-based selection across multiple domains in one secret.
5. Default certificate selection when the handshake carries no SNI.
6. Graceful fallback to self-signed at startup and per-handshake.
7. Last-good-certificate retention on reload failure.

### Out of Scope

- certbot certificate generation (handled by a separate external process).
- Uploading certificates to Secrets Manager (external process).
- Changes to the `tls.mode = none` path (NLB/proxy TLS termination).
- Client-side certificate validation behavior (cannot be controlled server-side).

## SECRET FORMAT

The Secrets Manager secret value is a JSON object keyed by site name. Each entry
holds a PEM full chain and a PEM private key:

```json
{
  "smtp.example.com": {
    "cert": "<fullchain.pem contents>",
    "key": "<privkey.pem contents>"
  }
}
```

Generated externally with:

```sh
jq -n --arg name "$DOMAIN" \
  --rawfile cert "$LIVE/fullchain.pem" \
  --rawfile key "$LIVE/privkey.pem" \
  '{ ($name): { cert: $cert, key: $key } }' > sm_value.json
```

The secret MAY contain multiple top-level keys (multi-domain). A single domain
is the common case (≈99%); multiple domains occur only when several hostnames
point at the same NLB (e.g. test domains).

## ENVIRONMENT VARIABLES

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `SMTP_PROXY_TLS_SECRET_ID` | No | `""` | Secrets Manager secret ID/ARN holding the certificate JSON. If empty, the provider is disabled and self-signed is used. |
| `SMTP_PROXY_TLS_RELOAD_INTERVAL` | No | `168` | Reload interval in **whole hours**. Default 168 (7 days). |
| `SMTP_PROXY_TLS_DEFAULT_CERT` | No | `""` | Domain key whose certificate is served when a handshake carries no SNI. If empty, the first key in sorted order is used. |

## EARS REQUIREMENTS

### Ubiquitous

- U1: The system SHALL expose the TLS certificate to the SMTP server through a
  `tls.Config.GetCertificate` callback so certificates can change without
  restarting the server.
- U2: The system SHALL keep the `tls.mode = none` path unchanged.

### Event-driven

- E1: WHEN `SMTP_PROXY_TLS_SECRET_ID` is set at startup, the system SHALL fetch
  and parse the secret and serve its certificates.
- E2: WHEN a TLS handshake includes an SNI server name that matches a loaded
  certificate domain, the system SHALL serve that domain's certificate.
- E3: WHEN a TLS handshake includes an SNI server name with no matching loaded
  certificate, the system SHALL serve a self-signed certificate for that
  handshake.
- E4: WHEN a TLS handshake carries no SNI server name, the system SHALL serve
  the default certificate (`SMTP_PROXY_TLS_DEFAULT_CERT` if set, otherwise the
  first domain in sorted order), or a self-signed certificate if no Secrets
  Manager certificate is loaded.
- E5: WHEN the reload interval elapses, the system SHALL re-fetch the secret and
  atomically replace the active certificate set on success.

### State-driven

- S1: WHILE `SMTP_PROXY_TLS_SECRET_ID` is empty, the system SHALL serve a
  self-signed certificate for all handshakes.
- S2: WHILE the most recent reload has failed, the system SHALL continue serving
  the last successfully loaded certificate set (or self-signed if none was ever
  loaded) and SHALL retry on the next interval.

### Unwanted behavior

- N1: IF a Secrets Manager fetch or parse fails at startup, THEN the system SHALL
  log a warning and fall back to self-signed without terminating.
- N2: IF an individual domain entry in the secret has an invalid cert/key pair,
  THEN the system SHALL skip only that entry and serve the remaining valid
  entries.
- N3: IF a reload fails after certificates were previously loaded, THEN the
  system SHALL NOT downgrade to self-signed and SHALL retain the last good set.

### Optional

- O1: WHERE `SMTP_PROXY_TLS_RELOAD_INTERVAL` is unset, the system SHALL default
  to 168 hours.

## ARCHITECTURE

```
[reload goroutine: every N hours]
    fetch secret -> parse JSON -> build per-domain tls.Certificate (validate)
    on success: atomic.Pointer.Store(new bundle)   (skip invalid entries)
    on failure: keep current bundle, log, retry next interval

[TLS handshake] tls.Config.GetCertificate(hello)
    bundle := atomic.Pointer.Load()
    hello.ServerName present -> bundle.byDomain[name] or self-signed
    hello.ServerName empty   -> bundle.default or self-signed
```

- Concurrency: a single `atomic.Pointer` holding an immutable certificate
  bundle. `GetCertificate` performs a lock-free read; the reload goroutine
  replaces the whole pointer. Verified under `go test -race`.
- Lifecycle: the reload goroutine is bound to a `context.Context` cancelled on
  server shutdown.
- The self-signed certificate is generated once at startup and reused as the
  fallback for all unmatched handshakes.

## IMPLEMENTATION UNITS (commit boundaries)

1. `internal/config`: add `SecretID`, `ReloadInterval`, `DefaultCert` to
   `TLSConfig`; defaults and env binding.
2. `internal/tlsutil`: Secrets Manager fetcher (interface, mockable), JSON
   parser/validator, atomic certificate bundle, reload loop, `GetCertificate`.
3. `cmd/smtp-server`: wire the provider into `s.TLSConfig`; `go.mod` dependency.

## DEPENDENCIES

- `github.com/aws/aws-sdk-go-v2/service/secretsmanager` (new).
- `aws-sdk-go-v2/config` and `credentials` are already present.
- AWS IAM: the ECS task role already grants `secretsmanager:GetSecretValue`.
