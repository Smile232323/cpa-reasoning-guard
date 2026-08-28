package main

import (
	"encoding/json"
	"testing"
)

func TestGuardReasoningResponsesRepairsDeclaredReasoning(t *testing.T) {
	body := guardReasoningBody([]byte(`{"model":"any-provider-model","reasoning":{}}`), "responses", "high")
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	reasoning, ok := payload["reasoning"].(map[string]any)
	if !ok || reasoning["effort"] != "high" {
		t.Fatalf("reasoning = %#v, want effort high", payload["reasoning"])
	}
}

func TestGuardReasoningResponsesReplacesZero(t *testing.T) {
	body := guardReasoningBody([]byte(`{"reasoning":{"effort":0}}`), "openai-responses", "xhigh")
	var payload map[string]map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["reasoning"]["effort"] != "xhigh" {
		t.Fatalf("effort = %#v, want xhigh", payload["reasoning"]["effort"])
	}
}

func TestGuardReasoningKeepsExplicitEffort(t *testing.T) {
	original := []byte(`{"reasoning":{"effort":"medium"}}`)
	body := guardReasoningBody(original, "responses", "high")
	if string(body) != string(original) {
		t.Fatalf("body changed = %s", body)
	}
}

func TestGuardReasoningChatRepairsZeroEffort(t *testing.T) {
	body := guardReasoningBody([]byte(`{"model":"any-provider-model","messages":[],"reasoning_effort":0}`), "chat-completions", "high")
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["reasoning_effort"] != "high" {
		t.Fatalf("reasoning_effort = %#v, want high", payload["reasoning_effort"])
	}
}

func TestGuardReasoningSkipsOtherFormats(t *testing.T) {
	original := []byte(`{"model":"any-provider-model","messages":[]}`)
	body := guardReasoningBody(original, "images", "high")
	if string(body) != string(original) {
		t.Fatalf("body changed = %s", body)
	}
}

func TestPluginRegistrationExposesManagementPageAndGlobalGuard(t *testing.T) {
	registration := pluginRegistration()
	if !registration.Capabilities.ModelProvider || !registration.Capabilities.ModelRouter || !registration.Capabilities.Executor {
		t.Fatalf("core capabilities = %#v", registration.Capabilities)
	}
	if !registration.Capabilities.RequestInterceptor || !registration.Capabilities.ManagementAPI {
		t.Fatalf("global guard and management page are not registered: %#v", registration.Capabilities)
	}
}

func TestManagementPageReturnsHTML(t *testing.T) {
	response, err := managementPage([]byte(`{"Method":"GET","Path":"/v0/resource/plugins/paratera-raw-responses/overview"}`))
	if err != nil {
		t.Fatal(err)
	}
	var envelope envelope
	if err := json.Unmarshal(response, &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.OK {
		t.Fatalf("management response failed: %#v", envelope.Error)
	}
	var page managementResponse
	if err := json.Unmarshal(envelope.Result, &page); err != nil {
		t.Fatal(err)
	}
	if page.StatusCode != 200 || len(page.Body) == 0 {
		t.Fatalf("page = %#v", page)
	}
}
