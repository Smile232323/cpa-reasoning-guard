package main

import (
	"encoding/json"
	"testing"
)

func TestGuardReasoningRepairsDeclaredMissingEffort(t *testing.T) {
	body := guardReasoningBody([]byte(`{"model":"any-provider-model","reasoning":{}}`), "high", true)
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	reasoning := payload["reasoning"].(map[string]any)
	if reasoning["effort"] != "high" {
		t.Fatalf("reasoning = %#v, want effort high", payload["reasoning"])
	}
}

func TestGuardReasoningReplacesExplicitZero(t *testing.T) {
	body := guardReasoningBody([]byte(`{"reasoning":{"effort":0}}`), "xhigh", true)
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
	if body := guardReasoningBody(original, "high", true); string(body) != string(original) {
		t.Fatalf("body changed = %s", body)
	}
}

func TestGuardReasoningChatRepairsZeroEffort(t *testing.T) {
	body := guardReasoningBody([]byte(`{"model":"any-provider-model","messages":[],"reasoning_effort":0}`), "high", true)
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["reasoning_effort"] != "high" {
		t.Fatalf("reasoning_effort = %#v, want high", payload["reasoning_effort"])
	}
}

func TestGuardReasoningDoesNotAddUnsupportedFields(t *testing.T) {
	original := []byte(`{"model":"provider-without-reasoning","messages":[]}`)
	if body := guardReasoningBody(original, "high", true); string(body) != string(original) {
		t.Fatalf("body changed = %s", body)
	}
}

func TestGuardCanLeaveMissingEffortUntouched(t *testing.T) {
	original := []byte(`{"reasoning":{}}`)
	if body := guardReasoningBody(original, "high", false); string(body) != string(original) {
		t.Fatalf("body changed = %s", body)
	}
}

func TestPluginRegistrationIsProviderAgnostic(t *testing.T) {
	registration := pluginRegistration()
	if !registration.Capabilities.RequestInterceptor || !registration.Capabilities.ManagementAPI {
		t.Fatalf("capabilities = %#v", registration.Capabilities)
	}
	if registration.Metadata.Name != "CPA Provider Reasoning Guard" {
		t.Fatalf("metadata = %#v", registration.Metadata)
	}
}

func TestManagementPageReturnsHTML(t *testing.T) {
	response, err := managementPage([]byte(`{"Method":"GET"}`))
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
