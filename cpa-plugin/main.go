package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	void* ptr;
	size_t len;
} cliproxy_buffer;

typedef int (*cliproxy_host_call_fn)(void*, const char*, const uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_host_free_fn)(void*, size_t);

typedef struct {
	uint32_t abi_version;
	void* host_ctx;
	cliproxy_host_call_fn call;
	cliproxy_host_free_fn free_buffer;
} cliproxy_host_api;

typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);

typedef struct {
	uint32_t abi_version;
	cliproxy_plugin_call_fn call;
	cliproxy_plugin_free_fn free_buffer;
	cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;

extern int cliproxyPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void cliproxyPluginFree(void*, size_t);
extern void cliproxyPluginShutdown(void);

static const cliproxy_host_api* stored_host;

static void store_host_api(const cliproxy_host_api* host) {
	stored_host = host;
}

static int call_host_api(const char* method, const uint8_t* request, size_t request_len, cliproxy_buffer* response) {
	if (stored_host == NULL || stored_host->call == NULL) {
		return 1;
	}
	return stored_host->call(stored_host->host_ctx, method, request, request_len, response);
}

static void free_host_buffer(void* ptr, size_t len) {
	if (stored_host != NULL && stored_host->free_buffer != NULL && ptr != NULL) {
		stored_host->free_buffer(ptr, len);
	}
}
*/
import "C"

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"
)

const pluginIdentifier = "paratera-raw-responses"

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type lifecycleRequest struct {
	ConfigYAML []byte `json:"config_yaml"`
}

type pluginConfig struct {
	Enabled             bool
	RawResponsesRouting bool
	UpstreamBaseURL     string
	APIKeyEnv           string
	ModelAliases        map[string]string
	ModelPrefixes       []string
	ReasoningGuard      bool
	DefaultEffort       string
}

type metadata struct {
	Name             string
	Version          string
	Author           string
	GitHubRepository string
	Logo             string
	ConfigFields     []configField
}

type configField struct {
	Name        string
	Type        string
	EnumValues  []string
	Description string
}

type registration struct {
	SchemaVersion uint32       `json:"schema_version"`
	Metadata      metadata     `json:"metadata"`
	Capabilities  capabilities `json:"capabilities"`
}

type capabilities struct {
	ModelProvider         bool     `json:"model_provider"`
	ModelRouter           bool     `json:"model_router"`
	Executor              bool     `json:"executor"`
	RequestInterceptor    bool     `json:"request_interceptor"`
	ManagementAPI         bool     `json:"management_api"`
	ExecutorModelScope    string   `json:"executor_model_scope"`
	ExecutorInputFormats  []string `json:"executor_input_formats"`
	ExecutorOutputFormats []string `json:"executor_output_formats"`
}

type modelRouteRequest struct {
	SourceFormat   string `json:"SourceFormat"`
	RequestedModel string `json:"RequestedModel"`
}

type modelRouteResponse struct {
	Handled    bool   `json:"Handled"`
	TargetKind string `json:"TargetKind"`
	Target     string `json:"Target"`
	Reason     string `json:"Reason"`
}

type modelInfo struct {
	ID                         string           `json:"ID"`
	Object                     string           `json:"Object"`
	OwnedBy                    string           `json:"OwnedBy"`
	Type                       string           `json:"Type"`
	DisplayName                string           `json:"DisplayName"`
	Name                       string           `json:"Name"`
	Description                string           `json:"Description"`
	ContextLength              int64            `json:"ContextLength"`
	MaxCompletionTokens        int64            `json:"MaxCompletionTokens"`
	SupportedGenerationMethods []string         `json:"SupportedGenerationMethods"`
	SupportedParameters        []string         `json:"SupportedParameters"`
	SupportedInputModalities   []string         `json:"SupportedInputModalities"`
	SupportedOutputModalities  []string         `json:"SupportedOutputModalities"`
	Thinking                   *thinkingSupport `json:"Thinking"`
	UserDefined                bool             `json:"UserDefined"`
}

type thinkingSupport struct {
	DynamicAllowed bool     `json:"DynamicAllowed"`
	Levels         []string `json:"Levels"`
}

type modelResponse struct {
	Provider string      `json:"Provider"`
	Models   []modelInfo `json:"Models"`
}

type executorRequest struct {
	AuthID          string      `json:"AuthID"`
	AuthProvider    string      `json:"AuthProvider"`
	Model           string      `json:"Model"`
	Format          string      `json:"Format"`
	Stream          bool        `json:"Stream"`
	Headers         http.Header `json:"Headers"`
	Query           url.Values  `json:"Query"`
	OriginalRequest []byte      `json:"OriginalRequest"`
	SourceFormat    string      `json:"SourceFormat"`
	Payload         []byte      `json:"Payload"`
	StreamID        string      `json:"stream_id,omitempty"`
}

type executorResponse struct {
	Payload  []byte      `json:"Payload,omitempty"`
	Headers  http.Header `json:"Headers,omitempty"`
	Metadata interface{} `json:"Metadata,omitempty"`
}

type executorStreamResponse struct {
	Headers http.Header `json:"headers,omitempty"`
}

type requestInterceptRequest struct {
	SourceFormat   string         `json:"SourceFormat"`
	ToFormat       string         `json:"ToFormat"`
	Model          string         `json:"Model"`
	RequestedModel string         `json:"RequestedModel"`
	Stream         bool           `json:"Stream"`
	Headers        http.Header    `json:"Headers"`
	Body           []byte         `json:"Body"`
	Metadata       map[string]any `json:"Metadata"`
}

type requestInterceptResponse struct {
	Headers      http.Header `json:"Headers,omitempty"`
	Body         []byte      `json:"Body,omitempty"`
	ClearHeaders []string    `json:"ClearHeaders,omitempty"`
}

type managementRegistrationResponse struct {
	Resources []resourceRoute `json:"resources,omitempty"`
}

type resourceRoute struct {
	Path        string `json:"Path"`
	Menu        string `json:"Menu"`
	Description string `json:"Description"`
}

type managementRequest struct {
	Method string `json:"Method"`
	Path   string `json:"Path"`
}

type managementResponse struct {
	StatusCode int         `json:"StatusCode"`
	Headers    http.Header `json:"Headers"`
	Body       []byte      `json:"Body"`
}

type hostStreamEmitRequest struct {
	StreamID string `json:"StreamID"`
	Payload  []byte `json:"Payload,omitempty"`
	Error    string `json:"Error,omitempty"`
}

type hostStreamCloseRequest struct {
	StreamID string `json:"StreamID"`
	Error    string `json:"Error,omitempty"`
}

var currentConfig atomic.Value

func main() {}

//export cliproxy_plugin_init
func cliproxy_plugin_init(host *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if plugin == nil {
		return 1
	}
	C.store_host_api(host)
	plugin.abi_version = C.uint32_t(1)
	plugin.call = C.cliproxy_plugin_call_fn(C.cliproxyPluginCall)
	plugin.free_buffer = C.cliproxy_plugin_free_fn(C.cliproxyPluginFree)
	plugin.shutdown = C.cliproxy_plugin_shutdown_fn(C.cliproxyPluginShutdown)
	return 0
}

//export cliproxyPluginCall
func cliproxyPluginCall(method *C.char, request *C.uint8_t, requestLen C.size_t, response *C.cliproxy_buffer) C.int {
	if response != nil {
		response.ptr = nil
		response.len = 0
	}
	if method == nil {
		writeResponse(response, errorEnvelope("invalid_method", "method is required"))
		return 1
	}
	var requestBytes []byte
	if request != nil && requestLen > 0 {
		requestBytes = C.GoBytes(unsafe.Pointer(request), C.int(requestLen))
	}
	raw, errHandle := handleMethod(C.GoString(method), requestBytes)
	if errHandle != nil {
		writeResponse(response, errorEnvelope("plugin_error", errHandle.Error()))
		return 1
	}
	writeResponse(response, raw)
	return 0
}

//export cliproxyPluginFree
func cliproxyPluginFree(ptr unsafe.Pointer, _ C.size_t) {
	if ptr != nil {
		C.free(ptr)
	}
}

//export cliproxyPluginShutdown
func cliproxyPluginShutdown() {}

func handleMethod(method string, request []byte) ([]byte, error) {
	switch method {
	case "plugin.register", "plugin.reconfigure":
		if err := configure(request); err != nil {
			return nil, err
		}
		return okEnvelope(pluginRegistration())
	case "model.static", "model.for_auth":
		return okEnvelope(staticModels())
	case "model.route":
		return routeModel(request)
	case "request.intercept_before", "request.intercept_after":
		return interceptRequest(request)
	case "management.register":
		return okEnvelope(managementRegistrationResponse{Resources: []resourceRoute{{
			Path:        "/overview",
			Menu:        "Paratera Raw Responses",
			Description: "Provider routing and global reasoning guard status.",
		}}})
	case "management.handle":
		return managementPage(request)
	case "executor.identifier":
		return okEnvelope(map[string]string{"identifier": pluginIdentifier})
	case "executor.execute":
		return execute(request)
	case "executor.execute_stream":
		return executeStream(request)
	case "executor.count_tokens":
		return okEnvelope(executorResponse{Payload: []byte(`{"input_tokens":0}`)})
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func defaultConfig() pluginConfig {
	return pluginConfig{
		Enabled:         true,
		RawResponsesRouting: true,
		UpstreamBaseURL: "https://llmapi.paratera.com/v1",
		APIKeyEnv:       "PARATERA_RAW_RESPONSES_API_KEY",
		ModelAliases: map[string]string{
			"gpt-5.6-luna":  "GPT-5.6-Luna",
			"gpt-5.6-sol":   "GPT-5.6-Sol",
			"gpt-5.6-terra": "GPT-5.6-Terra",
		},
		ModelPrefixes:  []string{"gpt-5.6-"},
		ReasoningGuard: true,
		DefaultEffort:  "high",
	}
}

func configure(raw []byte) error {
	cfg := defaultConfig()
	var lifecycle lifecycleRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &lifecycle); err != nil {
			return err
		}
	}
	for _, line := range strings.Split(string(lifecycle.ConfigYAML), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "enabled:") {
			cfg.Enabled = strings.TrimSpace(strings.TrimPrefix(trimmed, "enabled:")) != "false"
		}
		if strings.HasPrefix(trimmed, "upstream_base_url:") {
			cfg.UpstreamBaseURL = cleanValue(strings.TrimPrefix(trimmed, "upstream_base_url:"))
		}
		if strings.HasPrefix(trimmed, "raw_responses_routing:") {
			cfg.RawResponsesRouting = cleanValue(strings.TrimPrefix(trimmed, "raw_responses_routing:")) != "false"
		}
		if strings.HasPrefix(trimmed, "api_key_env:") {
			cfg.APIKeyEnv = cleanValue(strings.TrimPrefix(trimmed, "api_key_env:"))
		}
		if strings.HasPrefix(trimmed, "reasoning_guard:") {
			cfg.ReasoningGuard = cleanValue(strings.TrimPrefix(trimmed, "reasoning_guard:")) != "false"
		}
		if strings.HasPrefix(trimmed, "default_reasoning_effort:") {
			cfg.DefaultEffort = cleanValue(strings.TrimPrefix(trimmed, "default_reasoning_effort:"))
		}
		if strings.HasPrefix(trimmed, "- ") && strings.Contains(trimmed, "gpt-") {
			value := cleanValue(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
			if value != "" && !strings.Contains(value, ":") {
				cfg.ModelPrefixes = append(cfg.ModelPrefixes, value)
			}
		}
	}
	if strings.TrimSpace(cfg.UpstreamBaseURL) == "" {
		return fmt.Errorf("upstream_base_url is empty")
	}
	currentConfig.Store(cfg)
	return nil
}

func cleanValue(value string) string {
	return strings.Trim(strings.TrimSpace(value), "\"'")
}

func loadedConfig() pluginConfig {
	if value := currentConfig.Load(); value != nil {
		if cfg, ok := value.(pluginConfig); ok {
			return cfg
		}
	}
	return defaultConfig()
}

func pluginRegistration() registration {
	cfg := loadedConfig()
	routingEnabled := cfg.Enabled && cfg.RawResponsesRouting
	inputFormats := []string(nil)
	outputFormats := []string(nil)
	if routingEnabled {
		inputFormats = []string{"responses"}
		outputFormats = []string{"responses"}
	}
	return registration{
		SchemaVersion: 1,
		Metadata: metadata{
			Name:             "Paratera Raw Responses for CPA",
			Version:          "0.1.0",
			Author:           "Smile232323",
			GitHubRepository: "https://github.com/Smile232323/cpa-raw-responses-plugin",
			ConfigFields: []configField{
				{Name: "enabled", Type: "boolean", Description: "Enable automatic routing for configured provider models."},
				{Name: "raw_responses_routing", Type: "boolean", Description: "Enable the plugin-owned direct executor. Keep false when CPA's AI provider UI must own routing and credentials."},
				{Name: "upstream_base_url", Type: "string", Description: "OpenAI Responses-compatible upstream base URL."},
				{Name: "api_key_env", Type: "string", Description: "Environment variable containing the upstream API key."},
				{Name: "model_prefixes", Type: "array", Description: "Model prefixes that should route through this provider."},
				{Name: "reasoning_guard", Type: "boolean", Description: "Preserve a non-zero reasoning effort for every CPA provider/model request."},
				{Name: "default_reasoning_effort", Type: "string", Description: "Effort inserted when reasoning is absent or zero. Defaults to high."},
			},
		},
		Capabilities: capabilities{
			ModelProvider:         routingEnabled,
			ModelRouter:           routingEnabled,
			Executor:              routingEnabled,
			ExecutorModelScope:    map[bool]string{true: "static", false: ""}[routingEnabled],
			ExecutorInputFormats:  inputFormats,
			ExecutorOutputFormats: outputFormats,
			RequestInterceptor:    true,
			ManagementAPI:         true,
		},
	}
}

func managementPage(raw []byte) ([]byte, error) {
	var req managementRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	if strings.ToUpper(strings.TrimSpace(req.Method)) != http.MethodGet {
		return okEnvelope(managementResponse{StatusCode: http.StatusMethodNotAllowed, Headers: http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}}, Body: []byte("method not allowed")})
	}
	cfg := loadedConfig()
	page := fmt.Sprintf(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Paratera Raw Responses</title>
<style>
:root{color-scheme:light dark;font-family:Inter,ui-sans-serif,system-ui,sans-serif;background:#f7f8fa;color:#17202a}
body{margin:0;padding:28px;background:linear-gradient(135deg,#f8fafc,#eef3f8);min-height:100vh}
main{max-width:980px;margin:auto}.eyebrow{font-size:12px;letter-spacing:.12em;text-transform:uppercase;color:#637083;font-weight:700}
h1{margin:8px 0 6px;font-size:30px}p{color:#637083;line-height:1.6}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(240px,1fr));gap:14px;margin-top:24px}
.card{background:#fff;border:1px solid #dfe5ec;border-radius:16px;padding:18px;box-shadow:0 8px 24px #17202a0d}.label{font-size:12px;color:#637083}.value{font-weight:700;margin-top:7px;word-break:break-word}
.ok{color:#087f5b}.warn{color:#b45309}.models{display:flex;flex-wrap:wrap;gap:8px;margin-top:12px}.model{background:#e7f5ef;color:#087f5b;padding:7px 10px;border-radius:999px;font-size:13px;font-weight:700}
.links{display:flex;flex-wrap:wrap;gap:10px;margin-top:24px}a{color:#2563eb;text-decoration:none;font-weight:700;background:#fff;border:1px solid #dfe5ec;padding:10px 13px;border-radius:10px}
@media(prefers-color-scheme:dark){:root{background:#111827;color:#e5e7eb}body{background:linear-gradient(135deg,#111827,#172033)}.card,a{background:#182235;border-color:#334155}.label,p{color:#9aa8bb}.model{background:#123b31;color:#6ee7b7}}
</style></head><body><main>
<div class="eyebrow">CPA plugin · native router + executor + interceptor</div>
<h1>Paratera Raw Responses</h1>
	<p>Transparent Responses routing for configured Paratera models, plus a global guard that repairs explicit zero reasoning values across every CPA provider and model without adding unsupported parameters.</p>
<div class="grid">
<section class="card"><div class="label">Plugin status</div><div class="value %s">%s</div></section>
<section class="card"><div class="label">Global reasoning guard</div><div class="value %s">%s</div></section>
<section class="card"><div class="label">Default effort</div><div class="value">%s</div></section>
<section class="card"><div class="label">Raw upstream</div><div class="value">%s</div></section>
</div>
<section class="card" style="margin-top:14px"><div class="label">Raw Responses model aliases</div><div class="models">%s</div></section>
<div class="links"><a href="/management.html#/ai-providers">AI Providers</a><a href="/management.html#/logs">CPA Logs</a><a href="/v1/models">Model List</a><a href="http://62.234.89.241:8090/">Usage Dashboard</a></div>
</main></body></html>`, statusClass(cfg.Enabled), statusText(cfg.Enabled), statusClass(cfg.ReasoningGuard), reasoningText(cfg.ReasoningGuard), html.EscapeString(cfg.DefaultEffort), html.EscapeString(cfg.UpstreamBaseURL), modelBadges(cfg.ModelAliases))
	return okEnvelope(managementResponse{StatusCode: http.StatusOK, Headers: http.Header{"Content-Type": []string{"text/html; charset=utf-8"}}, Body: []byte(page)})
}

func statusClass(enabled bool) string {
	if enabled {
		return "ok"
	}
	return "warn"
}

func statusText(enabled bool) string {
	if enabled {
		return "Enabled"
	}
	return "Disabled"
}

func reasoningText(enabled bool) string {
	if enabled {
		return "Enabled for all providers and models"
	}
	return "Disabled"
}

func modelBadges(aliases map[string]string) string {
	models := make([]string, 0, len(aliases))
	for alias := range aliases {
		models = append(models, alias)
	}
	if len(models) == 0 {
		return `<span class="label">No aliases configured</span>`
	}
	sort.Strings(models)
	badges := make([]string, 0, len(models))
	for _, model := range models {
		badges = append(badges, `<span class="model">`+html.EscapeString(model)+`</span>`)
	}
	return strings.Join(badges, "")
}

func interceptRequest(raw []byte) ([]byte, error) {
	var req requestInterceptRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	cfg := loadedConfig()
	if !cfg.Enabled || !cfg.ReasoningGuard {
		return okEnvelope(requestInterceptResponse{})
	}
	effort := strings.TrimSpace(cfg.DefaultEffort)
	if effort == "" {
		effort = "high"
	}
	body := guardReasoningBody(req.Body, req.SourceFormat+" "+req.ToFormat, effort)
	if bytes.Equal(body, req.Body) {
		return okEnvelope(requestInterceptResponse{})
	}
	return okEnvelope(requestInterceptResponse{Body: body})
}

func guardReasoningBody(body []byte, format, effort string) []byte {
	if len(body) == 0 {
		return body
	}
	_ = format
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	changed := false
	if raw, hasReasoning := payload["reasoning"]; hasReasoning {
		var reasoning map[string]json.RawMessage
		if err := json.Unmarshal(raw, &reasoning); err != nil || reasoning == nil {
			reasoning = make(map[string]json.RawMessage)
		}
		if raw, ok := reasoning["effort"]; !ok || reasoningValueIsZero(raw) {
			reasoning["effort"], _ = json.Marshal(effort)
			changed = true
		}
		if changed {
			payload["reasoning"], _ = json.Marshal(reasoning)
		}
	}
	if raw, hasReasoningEffort := payload["reasoning_effort"]; hasReasoningEffort && reasoningValueIsZero(raw) {
		payload["reasoning_effort"], _ = json.Marshal(effort)
		changed = true
	}
	if !changed {
		return body
	}
	updated, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return updated
}

func reasoningValueIsZero(raw json.RawMessage) bool {
	value := strings.ToLower(strings.TrimSpace(string(raw)))
	return value == "" || value == "null" || value == "0" || value == "0.0" || value == "false" || value == `"0"` || value == `"0.0"`
}

func staticModels() modelResponse {
	cfg := loadedConfig()
	models := make([]modelInfo, 0, len(cfg.ModelAliases))
	for alias, upstream := range cfg.ModelAliases {
		models = append(models, modelInfo{
			ID:                         alias,
			Object:                     "model",
			OwnedBy:                    pluginIdentifier,
			Type:                       "responses",
			DisplayName:                alias + " (Paratera Raw)",
			Name:                       upstream,
			Description:                "Paratera Responses model routed by the CPA companion plugin.",
			ContextLength:              500000,
			MaxCompletionTokens:        128000,
			SupportedGenerationMethods: []string{"responses"},
			SupportedParameters:        []string{"reasoning.effort", "stream", "store", "max_output_tokens"},
			SupportedInputModalities:   []string{"text"},
			SupportedOutputModalities:  []string{"text"},
			Thinking:                   &thinkingSupport{DynamicAllowed: true, Levels: []string{"minimal", "low", "medium", "high", "xhigh"}},
			UserDefined:                true,
		})
	}
	return modelResponse{Provider: pluginIdentifier, Models: models}
}

func routeModel(raw []byte) ([]byte, error) {
	var req modelRouteRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	cfg := loadedConfig()
	if !cfg.Enabled || !isResponsesFormat(req.SourceFormat) || !matchesModel(cfg, req.RequestedModel) {
		return okEnvelope(modelRouteResponse{Handled: false})
	}
	return okEnvelope(modelRouteResponse{
		Handled:    true,
		TargetKind: "self",
		Target:     pluginIdentifier,
		Reason:     "provider_model_routed_to_raw_responses",
	})
}

func isResponsesFormat(format string) bool {
	format = strings.ToLower(strings.TrimSpace(format))
	return strings.Contains(format, "response")
}

func matchesModel(cfg pluginConfig, model string) bool {
	model = strings.TrimSpace(model)
	if _, ok := cfg.ModelAliases[strings.ToLower(model)]; ok {
		return true
	}
	for _, prefix := range cfg.ModelPrefixes {
		if strings.HasPrefix(strings.ToLower(model), strings.ToLower(strings.TrimSpace(prefix))) {
			return true
		}
	}
	return false
}

func execute(raw []byte) ([]byte, error) {
	var req executorRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	body := req.OriginalRequest
	if len(body) == 0 {
		body = req.Payload
	}
	response, err := upstreamRequest(context.Background(), req.Headers, req.Query, body)
	if err != nil {
		return errorEnvelope("executor_error", err.Error()), nil
	}
	return okEnvelope(executorResponse{Payload: response.Body, Headers: response.Headers})
}

func executeStream(raw []byte) ([]byte, error) {
	var req executorRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.StreamID) == "" {
		return errorEnvelope("executor_error", "stream_id is required"), nil
	}
	go streamUpstream(req)
	return okEnvelope(executorStreamResponse{Headers: http.Header{"Content-Type": []string{"text/event-stream"}}})
}

type upstreamResponse struct {
	Body    []byte
	Headers http.Header
}

func upstreamRequest(ctx context.Context, inbound http.Header, query url.Values, body []byte) (upstreamResponse, error) {
	cfg := loadedConfig()
	key := strings.TrimSpace(os.Getenv(cfg.APIKeyEnv))
	if key == "" {
		return upstreamResponse{}, fmt.Errorf("environment variable %s is empty", cfg.APIKeyEnv)
	}
	body = mapModelAlias(body, cfg.ModelAliases)
	endpoint := strings.TrimRight(cfg.UpstreamBaseURL, "/") + "/responses"
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return upstreamResponse{}, err
	}
	copyHeaders(request.Header, inbound)
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Content-Type", "application/json")
	client := directHTTPClient()
	response, err := client.Do(request)
	if err != nil {
		return upstreamResponse{}, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return upstreamResponse{}, err
	}
	if response.StatusCode >= 400 {
		return upstreamResponse{}, fmt.Errorf("upstream returned HTTP %d: %s", response.StatusCode, summarizeError(responseBody))
	}
	return upstreamResponse{Body: responseBody, Headers: cloneHeaders(response.Header)}, nil
}

func streamUpstream(req executorRequest) {
	streamID := req.StreamID
	fail := func(err error) {
		closePluginStream(streamID, err.Error())
	}
	cfg := loadedConfig()
	key := strings.TrimSpace(os.Getenv(cfg.APIKeyEnv))
	if key == "" {
		fail(fmt.Errorf("environment variable %s is empty", cfg.APIKeyEnv))
		return
	}
	body := req.OriginalRequest
	if len(body) == 0 {
		body = req.Payload
	}
	body = mapModelAlias(body, cfg.ModelAliases)
	endpoint := strings.TrimRight(cfg.UpstreamBaseURL, "/") + "/responses"
	if encoded := req.Query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		fail(err)
		return
	}
	copyHeaders(request.Header, req.Headers)
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Content-Type", "application/json")
	response, err := directHTTPClient().Do(request)
	if err != nil {
		fail(err)
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		body, _ := io.ReadAll(response.Body)
		fail(fmt.Errorf("upstream returned HTTP %d: %s", response.StatusCode, summarizeError(body)))
		return
	}
	buffer := make([]byte, 32*1024)
	for {
		count, readErr := response.Body.Read(buffer)
		if count > 0 {
			if errEmit := emitPluginStreamChunk(streamID, bytes.Clone(buffer[:count])); errEmit != nil {
				fail(errEmit)
				return
			}
		}
		if readErr == io.EOF {
			closePluginStream(streamID, "")
			return
		}
		if readErr != nil {
			fail(readErr)
			return
		}
	}
}

func directHTTPClient() *http.Client {
	transport := &http.Transport{Proxy: nil, TLSHandshakeTimeout: 20 * time.Second}
	return &http.Client{Transport: transport, Timeout: 5 * time.Minute}
}

func mapModelAlias(body []byte, aliases map[string]string) []byte {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	var model string
	if err := json.Unmarshal(payload["model"], &model); err != nil {
		return body
	}
	if mapped, ok := aliases[strings.ToLower(model)]; ok {
		payload["model"], _ = json.Marshal(mapped)
		body, _ = json.Marshal(payload)
	}
	return body
}

func copyHeaders(dst, src http.Header) {
	hopByHop := map[string]bool{"authorization": true, "connection": true, "content-length": true, "host": true, "transfer-encoding": true, "accept-encoding": true}
	for key, values := range src {
		if hopByHop[strings.ToLower(key)] {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func cloneHeaders(src http.Header) http.Header {
	dst := make(http.Header, len(src))
	for key, values := range src {
		dst[key] = append([]string(nil), values...)
	}
	return dst
}

func summarizeError(body []byte) string {
	if len(body) > 512 {
		body = body[:512]
	}
	return strings.ReplaceAll(strings.TrimSpace(string(body)), "\n", " ")
}

func emitPluginStreamChunk(streamID string, payload []byte) error {
	_, err := callHost("host.stream.emit", hostStreamEmitRequest{StreamID: streamID, Payload: payload})
	return err
}

func closePluginStream(streamID, errorMessage string) {
	_, _ = callHost("host.stream.close", hostStreamCloseRequest{StreamID: streamID, Error: errorMessage})
}

func callHost(method string, request any) ([]byte, error) {
	rawRequest, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	cMethod := C.CString(method)
	defer C.free(unsafe.Pointer(cMethod))
	var requestPtr *C.uint8_t
	if len(rawRequest) > 0 {
		requestPtr = (*C.uint8_t)(C.CBytes(rawRequest))
		defer C.free(unsafe.Pointer(requestPtr))
	}
	var response C.cliproxy_buffer
	if C.call_host_api(cMethod, requestPtr, C.size_t(len(rawRequest)), &response) != 0 {
		return nil, fmt.Errorf("host call failed: %s", method)
	}
	if response.ptr == nil || response.len == 0 {
		return nil, nil
	}
	defer C.free_host_buffer(response.ptr, response.len)
	return C.GoBytes(response.ptr, C.int(response.len)), nil
}

func okEnvelope(value any) ([]byte, error) {
	result, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope{OK: true, Result: result})
}

func errorEnvelope(code, message string) []byte {
	digest := sha256.Sum256([]byte(message))
	message = message + " [" + hex.EncodeToString(digest[:])[:12] + "]"
	raw, _ := json.Marshal(envelope{OK: false, Error: &envelopeError{Code: code, Message: message}})
	return raw
}

func writeResponse(response *C.cliproxy_buffer, raw []byte) {
	if response == nil || len(raw) == 0 {
		return
	}
	ptr := C.CBytes(raw)
	if ptr == nil {
		return
	}
	response.ptr = ptr
	response.len = C.size_t(len(raw))
}
