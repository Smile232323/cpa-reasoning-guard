# CPA Raw Responses Plugin

**An independent native plugin for [CLIProxyAPI / CPA](https://github.com/router-for-me/CLIProxyAPI).**

It keeps clients on CPA's normal `:8317` endpoint while providing two protections:

1. **CPA-managed Paratera GPT-5.6 routing.** Configure Paratera as a CPA `openai-compatibility` provider and map the lower-case aliases to Paratera's case-sensitive model IDs (`GPT-5.6-Terra`, etc.). Its `disabled` setting in CPAMC's **AI Providers** screen is the routing switch.
2. **A global reasoning guard for every CPA provider and model.** The plugin runs as a CPA request interceptor and repairs only an explicit zero, null, or empty `reasoning.effort` / `reasoning_effort` value. Existing non-zero effort is preserved. It does **not** add reasoning fields to requests that did not declare them, preventing incompatible models from receiving unsupported parameters.

> This is not an official CLIProxyAPI distribution. It is a CPA native plugin plus an optional legacy sidecar fallback.

## CPAMC Frontend

After installation, CPA automatically lists **Paratera Raw Responses** in:

```text
http://your-cpa-host:8317/management.html#/plugin-pages/
```

The plugin page shows routing status, global reasoning-guard status, the default repair effort, upstream base URL, and the registered Paratera model aliases.

## Features

- Native CPA `request_interceptor` and `management_api` capabilities, with an optional plugin-owned Raw Responses executor.
- Normal CPA entrypoint at `http://your-cpa-host:8317/v1`.
- CPA management UI registration and plugin resource page.
- CPA request/usage handling remains in the normal gateway path.
- Responses request body preservation for the selected Paratera model family.
- The recommended provider uses `proxy-url: direct` and is managed by CPA itself.
- Case-sensitive Paratera mapping for lower-case `gpt-5.6-luna`, `gpt-5.6-sol`, and `gpt-5.6-terra` aliases.
- Non-streaming and SSE streaming support.
- No API-key or request-body logging.

## Build and Release

```bash
make test
make release
```

The release artifact is written to:

```text
dist/paratera-raw-responses-v0.1.0.so
dist/checksums.txt
```

The target platform is `linux/amd64`, matching a standard x86_64 CPA VPS.

## Install on CPA

1. Copy `dist/paratera-raw-responses-v0.1.0.so` to:

   ```text
   /opt/cliproxy/plugins/linux/amd64/paratera-raw-responses-v0.1.0.so
   ```

2. Merge `plugin-config.example.yaml` and the `openai-compatibility` provider below into CPA's `config.yaml`. Keep `raw_responses_routing: false` so the AI Provider toggle remains authoritative.

   ```yaml
   openai-compatibility:
     - name: paratera-raw-responses
       disabled: false
       priority: 100
       base-url: https://llmapi.paratera.com/v1
       api-key-entries:
         - api-key: <PARATERA_API_KEY>
           proxy-url: direct
       models:
         - { name: GPT-5.6-Luna, alias: gpt-5.6-luna, force-mapping: true }
         - { name: GPT-5.6-Sol, alias: gpt-5.6-sol, force-mapping: true }
         - { name: GPT-5.6-Terra, alias: gpt-5.6-terra, force-mapping: true }
   ```

3. Restart CPA:

   ```bash
   sudo systemctl daemon-reload
   sudo systemctl restart cliproxy.service
   ```

4. Verify the AI Provider toggle in CPAMC, `GET /v1/models`, `POST /v1/responses`, and the CPA usage dashboard.

## Configuration

See `plugin-config.example.yaml` and `cpa-plugin/README.md`.

`reasoning_guard: true` applies to all CPA providers and models. It repairs **declared zero/empty reasoning values** but intentionally leaves requests without a reasoning field unchanged for provider compatibility.

## Optional Legacy Sidecar

`paratera_raw_relay.py` and `paratera-raw-relay.service` are retained only as a fallback for environments that cannot load native CPA plugins. The native plugin is the recommended deployment because it appears in CPAMC and preserves CPA-side visibility.

## Security

- CPA's root-owned `config.yaml` contains the provider key; never commit it.
- Do not commit `.env`, generated release artifacts, or real API keys.
- Validate the CPA YAML and back up `/opt/cliproxy/config.yaml` before restarting.

## License

MIT. See [LICENSE](LICENSE).
