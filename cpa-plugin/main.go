package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct { void* ptr; size_t len; } cliproxy_buffer;
typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);
typedef struct { uint32_t abi_version; void* host_ctx; void* call; void* free_buffer; } cliproxy_host_api;
typedef struct { uint32_t abi_version; cliproxy_plugin_call_fn call; cliproxy_plugin_free_fn free_buffer; cliproxy_plugin_shutdown_fn shutdown; } cliproxy_plugin_api;
extern int cliproxyPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void cliproxyPluginFree(void*, size_t);
extern void cliproxyPluginShutdown(void);
*/
import "C"

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"sync/atomic"
	"unsafe"
)

const (
	pluginID      = "cpa-reasoning-guard"
	pluginVersion = "0.2.0"
)

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
	ReasoningGuard      bool
	RepairMissingEffort bool
	DefaultEffort       string
}

type metadata struct {
	Name             string        `json:"Name"`
	Version          string        `json:"Version"`
	Author           string        `json:"Author"`
	GitHubRepository string        `json:"GitHubRepository"`
	ConfigFields     []configField `json:"ConfigFields"`
}

type configField struct {
	Name        string   `json:"Name"`
	Type        string   `json:"Type"`
	EnumValues  []string `json:"EnumValues,omitempty"`
	Description string   `json:"Description"`
}

type registration struct {
	SchemaVersion uint32       `json:"schema_version"`
	Metadata      metadata     `json:"metadata"`
	Capabilities  capabilities `json:"capabilities"`
}

type capabilities struct {
	RequestInterceptor bool `json:"request_interceptor"`
	ManagementAPI      bool `json:"management_api"`
}

type requestInterceptRequest struct {
	Body []byte `json:"Body"`
}
type requestInterceptResponse struct {
	Body []byte `json:"Body,omitempty"`
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
}
type managementResponse struct {
	StatusCode int         `json:"StatusCode"`
	Headers    http.Header `json:"Headers"`
	Body       []byte      `json:"Body"`
}

var currentConfig atomic.Value

func main() {}

//export cliproxy_plugin_init
func cliproxy_plugin_init(_ *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if plugin == nil {
		return 1
	}
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
	raw, err := handleMethod(C.GoString(method), requestBytes)
	if err != nil {
		writeResponse(response, errorEnvelope("plugin_error", err.Error()))
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
	case "request.intercept_before", "request.intercept_after":
		return interceptRequest(request)
	case "management.register":
		return okEnvelope(managementRegistrationResponse{Resources: []resourceRoute{{Path: "/overview", Menu: "CPA Reasoning Guard", Description: "Global reasoning-effort repair status for CPA-managed providers."}}})
	case "management.handle":
		return managementPage(request)
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func defaultConfig() pluginConfig {
	return pluginConfig{Enabled: true, ReasoningGuard: true, RepairMissingEffort: true, DefaultEffort: "high"}
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
		key, value, found := strings.Cut(strings.TrimSpace(line), ":")
		if !found {
			continue
		}
		value = cleanValue(value)
		switch key {
		case "enabled":
			cfg.Enabled = value != "false"
		case "reasoning_guard":
			cfg.ReasoningGuard = value != "false"
		case "repair_missing_effort":
			cfg.RepairMissingEffort = value != "false"
		case "default_reasoning_effort":
			cfg.DefaultEffort = value
		}
	}
	if !validEffort(cfg.DefaultEffort) {
		return fmt.Errorf("default_reasoning_effort must be one of minimal, low, medium, high, xhigh")
	}
	currentConfig.Store(cfg)
	return nil
}

func cleanValue(value string) string { return strings.Trim(strings.TrimSpace(value), "\"'") }

func validEffort(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "minimal", "low", "medium", "high", "xhigh":
		return true
	default:
		return false
	}
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
	return registration{SchemaVersion: 1, Metadata: metadata{
		Name: "CPA Provider Reasoning Guard", Version: pluginVersion, Author: "Smile232323", GitHubRepository: "https://github.com/Smile232323/cpa-reasoning-guard",
		ConfigFields: []configField{
			{Name: "enabled", Type: "boolean", Description: "Enable the guard for all requests routed by enabled CPA providers."},
			{Name: "reasoning_guard", Type: "boolean", Description: "Repair only declared reasoning settings; never adds a new reasoning field."},
			{Name: "repair_missing_effort", Type: "boolean", Description: "Add the configured effort when a declared reasoning object has no effort value."},
			{Name: "default_reasoning_effort", Type: "string", EnumValues: []string{"minimal", "low", "medium", "high", "xhigh"}, Description: "Effort used to replace explicit zero, null, empty, or false values."},
		},
	}, Capabilities: capabilities{RequestInterceptor: true, ManagementAPI: true}}
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
	body := guardReasoningBody(req.Body, cfg.DefaultEffort, cfg.RepairMissingEffort)
	if bytes.Equal(body, req.Body) {
		return okEnvelope(requestInterceptResponse{})
	}
	return okEnvelope(requestInterceptResponse{Body: body})
}

func guardReasoningBody(body []byte, effort string, repairMissing bool) []byte {
	if len(body) == 0 {
		return body
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	changed := false
	if raw, declared := payload["reasoning"]; declared {
		var reasoning map[string]json.RawMessage
		if err := json.Unmarshal(raw, &reasoning); err != nil || reasoning == nil {
			reasoning = make(map[string]json.RawMessage)
		}
		rawEffort, hasEffort := reasoning["effort"]
		if (!hasEffort && repairMissing) || (hasEffort && reasoningValueNeedsRepair(rawEffort)) {
			reasoning["effort"], _ = json.Marshal(effort)
			payload["reasoning"], _ = json.Marshal(reasoning)
			changed = true
		}
	}
	if raw, declared := payload["reasoning_effort"]; declared && reasoningValueNeedsRepair(raw) {
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

func reasoningValueNeedsRepair(raw json.RawMessage) bool {
	value := strings.ToLower(strings.TrimSpace(string(raw)))
	return value == "" || value == "null" || value == "0" || value == "0.0" || value == "false" || value == `""` || value == `"0"` || value == `"0.0"`
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
	page := fmt.Sprintf(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>CPA Provider Reasoning Guard</title><style>:root{font-family:Inter,ui-sans-serif,system-ui,sans-serif;color:#17202a;background:#f7f8fa}body{margin:0;padding:28px;background:linear-gradient(135deg,#f8fafc,#eef3f8);min-height:100vh}main{max-width:900px;margin:auto}.eyebrow{font-size:12px;letter-spacing:.12em;text-transform:uppercase;color:#637083;font-weight:700}h1{margin:8px 0 6px;font-size:30px}p{color:#637083;line-height:1.6}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:14px;margin-top:24px}.card{background:#fff;border:1px solid #dfe5ec;border-radius:16px;padding:18px;box-shadow:0 8px 24px #17202a0d}.label{font-size:12px;color:#637083}.value{font-weight:700;margin-top:7px}.ok{color:#087f5b}.warn{color:#b45309}.links{display:flex;flex-wrap:wrap;gap:10px;margin-top:24px}a{color:#2563eb;text-decoration:none;font-weight:700;background:#fff;border:1px solid #dfe5ec;padding:10px 13px;border-radius:10px}</style></head><body><main><div class="eyebrow">CPA native plugin · provider-agnostic interceptor</div><h1>CPA Provider Reasoning Guard</h1><p>Runs only inside CPA's request path. Every enabled CPA AI provider remains responsible for its own routing, credentials, aliases, logging, and enable/disable switch.</p><div class="grid"><section class="card"><div class="label">Plugin status</div><div class="value %s">%s</div></section><section class="card"><div class="label">Guard scope</div><div class="value">All CPA-routed providers and models</div></section><section class="card"><div class="label">Default effort</div><div class="value">%s</div></section><section class="card"><div class="label">Missing effort repair</div><div class="value">%s</div></section></div><section class="card" style="margin-top:14px"><div class="label">Safety behavior</div><p>Repairs only a declared <code>reasoning.effort</code> or <code>reasoning_effort</code> that is zero, null, empty, or false. It never creates reasoning fields for requests that did not declare them.</p></section><div class="links"><a href="/management.html#/ai-providers">AI Providers</a><a href="/management.html#/logs">CPA Logs</a><a href="/v1/models">Model List</a></div></main></body></html>`, statusClass(cfg.Enabled && cfg.ReasoningGuard), statusText(cfg.Enabled && cfg.ReasoningGuard), html.EscapeString(cfg.DefaultEffort), statusText(cfg.RepairMissingEffort))
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

func okEnvelope(value any) ([]byte, error) {
	result, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope{OK: true, Result: result})
}

func errorEnvelope(code, message string) []byte {
	digest := sha256.Sum256([]byte(message))
	message += " [" + hex.EncodeToString(digest[:])[:12] + "]"
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
