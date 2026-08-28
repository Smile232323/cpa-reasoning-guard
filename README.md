# CLIProxyAPI Raw Responses Plugin

**A CPA companion plugin/sidecar for transparent OpenAI Responses API routing.**

This small relay is designed for [CLIProxyAPI (CPA)](https://github.com/router-for-me/CLIProxyAPI) deployments that need the upstream behavior of a native OpenAI Responses endpoint. It preserves the request body, streaming responses, response status, and upstream headers instead of sending the request through CPA's Codex request normalizer.

The relay applies one provider-specific compatibility mapping: lower-case GPT-5.6 aliases are converted to Paratera's case-sensitive upstream model IDs.

> This is an independent CPA companion project, not an official CLIProxyAPI distribution or upstream plugin binary.

## Why

CPA's `codex-api-key` executor intentionally normalizes Responses requests for Codex-compatible upstreams. That is useful for Codex providers, but it can change provider-specific behavior. This sidecar is useful when a CPA deployment must retain the original Responses request semantics for one upstream.

## Features

- OpenAI Responses API forwarding under `/v1/*`.
- Non-streaming and SSE streaming support.
- Strict bearer-key authentication on API routes.
- Direct HTTPS connection to the configured upstream; no ambient proxy use.
- Case-sensitive model alias mapping for `gpt-5.6-luna`, `gpt-5.6-sol`, and `gpt-5.6-terra`.
- No request-body or API-key logging.
- Health endpoint at `/healthz`.

## Quick Start

```bash
cp .env.example .env
chmod 600 .env
python3 paratera_raw_relay.py
```

Set `UPSTREAM_API_KEY` to the upstream provider key. If `CLIENT_API_KEY` is omitted, the relay accepts the same key from clients and forwards it upstream.

Point Codex or another OpenAI Responses client to:

```text
http://your-host:8320/v1
```

Keep the client model lower-case when using the built-in aliases, for example `gpt-5.6-terra`.

## systemd

The repository includes `paratera-raw-relay.service` as a deployment template. Install the script under `/opt/paratera-raw-relay/`, keep credentials in a root-readable environment file, and enable the service:

```bash
sudo install -d -m 0755 /opt/paratera-raw-relay
sudo install -m 0755 paratera_raw_relay.py /opt/paratera-raw-relay/
sudo install -m 0644 paratera-raw-relay.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now paratera-raw-relay.service
```

Use a `root:root` environment file with mode `0600` at `/etc/paratera-raw-relay.env`.

## Security

- Use a dedicated upstream key where possible.
- Restrict the listener with a cloud firewall or bind it to a private network.
- Prefer HTTPS or an SSH/Tailscale private path when client traffic crosses an untrusted network.
- Do not commit `.env` or any real API key.

## License

MIT. See [LICENSE](LICENSE).
