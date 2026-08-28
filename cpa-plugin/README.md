# CPA Native Plugin

This is a native `linux/amd64` CPA plugin. Its recommended mode exposes two capabilities:

- **Request Interceptor** — runs around every CPA provider/model request and repairs an explicitly zero/empty reasoning setting while preserving non-zero client settings. It intentionally does not add a reasoning parameter to requests that never declared one, so non-reasoning models do not receive unsupported fields.
- **Management Page** — appears in CPAMC at `#/plugin-pages/paratera-raw-responses/0`.

Set `raw_responses_routing: true` only when you explicitly want the plugin-owned executor. In the recommended mode it is `false`: CPA's `openai-compatibility` Paratera provider owns aliases, credentials, and the AI Provider enable/disable toggle.

## Build

```bash
CGO_ENABLED=1 go build -buildmode=c-shared -o paratera-raw-responses-v0.1.0.so main.go
rm -f paratera-raw-responses-v0.1.0.h
```

Use Linux `amd64` for the VPS deployment. `make release` builds the release artifact in Docker.

## CPA Configuration

```yaml
plugins:
  enabled: true
  configs:
    paratera-raw-responses:
      enabled: true
      raw_responses_routing: false
      reasoning_guard: true
      default_reasoning_effort: high
```

Configure the Paratera API key in CPA's root-owned `openai-compatibility` provider entry, not in Git.
