# CPA Native Plugin

This `linux/amd64` native plugin exposes only two CPA capabilities:

- **Request Interceptor** — inspects requests after they enter CPA and repairs a declared invalid `reasoning.effort` / `reasoning_effort` value.
- **Management Page** — appears in CPAMC at `#/plugin-pages/cpa-reasoning-guard/0`.

It deliberately does **not** expose a model provider, model router, executor, static models, credential store, or direct HTTP client. CPA's existing enabled AI Providers retain full control of routing and credentials.

## Build

```bash
CGO_ENABLED=1 go build -buildmode=c-shared -o cpa-reasoning-guard-v0.2.0.so main.go
rm -f cpa-reasoning-guard-v0.2.0.h
```

Use Linux `amd64` for a standard x86_64 CPA VPS. `make release` builds the release artifact in Docker.

## CPA Configuration

```yaml
plugins:
  enabled: true
  configs:
    cpa-reasoning-guard:
      enabled: true
      reasoning_guard: true
      repair_missing_effort: true
      default_reasoning_effort: high
```

Provider credentials, base URLs, model aliases, and enable/disable state belong exclusively in CPA's normal AI Provider configuration.
