# CPA Provider Reasoning Guard

**A provider-agnostic native plugin for [CLIProxyAPI / CPA](https://github.com/router-for-me/CLIProxyAPI).**

CPA remains the only gateway. This plugin does not add an AI provider, contain an upstream URL, hold API keys, map model aliases, or make outbound model requests. Instead, it intercepts requests already routed by CPA to **any enabled AI Provider** and safely repairs malformed reasoning effort values.

## Why

Some intermediaries or clients serialize `reasoning.effort` / `reasoning_effort` as `0`, `null`, `false`, or an empty value. That can silently reduce reasoning quality even when the original provider supports an effort setting.

The guard fixes that single compatibility issue while preserving CPA's normal routing:

- Enable or disable a provider in CPAMC's **AI Providers** page to control whether it receives traffic.
- Keep each provider's own key, base URL, aliases, proxy policy, usage accounting, and logs in CPA.
- Use the same CPA `:8317` entrypoint and API token for every client.
- Apply the repair to every request routed by an enabled CPA provider, not to one hard-coded upstream.

## Safety rules

- Repairs only a **declared** `reasoning.effort` or `reasoning_effort` whose value is zero, null, false, or empty.
- Optionally fills a missing `effort` inside an existing `reasoning: {}` object.
- Preserves valid explicit efforts such as `low`, `medium`, `high`, and `xhigh`.
- Never adds a reasoning field to requests that did not already declare one, so providers/models without reasoning support remain compatible.
- Contains no credentials and performs no direct upstream calls.

## Install

1. Download `cpa-reasoning-guard-v0.2.0.so` from the release and copy it to your CPA plugin directory:

   ```bash
   sudo install -m 0755 cpa-reasoning-guard-v0.2.0.so \
     /opt/cliproxy/plugins/linux/amd64/cpa-reasoning-guard-v0.2.0.so
   ```

2. Merge `plugin-config.example.yaml` into CPA's `config.yaml`:

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

3. Restart CPA and open CPAMC:

   ```bash
   sudo systemctl restart cliproxy.service
   ```

   The plugin appears under `management.html#/plugin-pages/` as **CPA Reasoning Guard**.

## Configuration

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `true` | Enables this plugin only; it never changes AI Provider enablement. |
| `reasoning_guard` | `true` | Enables request interception. |
| `repair_missing_effort` | `true` | Fills `reasoning: {}` with the configured effort. |
| `default_reasoning_effort` | `high` | One of `minimal`, `low`, `medium`, `high`, `xhigh`. |

## Build

```bash
make test
make release
```

The release artifact is `dist/cpa-reasoning-guard-v0.2.0.so` for `linux/amd64` CPA hosts.

## License

MIT. See [LICENSE](LICENSE).
