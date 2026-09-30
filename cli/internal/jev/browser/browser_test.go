// Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

package browser

import (
	"errors"
	"strings"
	"testing"

	"github.com/qualcomm/GenieX/cli/internal/jev"
)

func TestLaunchArgsRestrictsCDPOrigin(t *testing.T) {
	args := launchArgs(9222, "profile", true, "https://example.test")
	joined := strings.Join(args, "\n")
	if !strings.Contains(joined, "--remote-allow-origins="+cdpClientOrigin) {
		t.Fatalf("launch args do not allow the client origin: %#v", args)
	}
	if strings.Contains(joined, "--remote-allow-origins=*") {
		t.Fatalf("launch args allow every origin: %#v", args)
	}
}

func TestDecodeEvaluateResult(t *testing.T) {
	for _, test := range []struct {
		name  string
		body  string
		value string
		stale bool
		ok    bool
	}{
		{"value", `{"result":{"value":"clicked"}}`, `"clicked"`, false, true},
		{"stale", `{"result":{"type":"undefined"},"exceptionDetails":{"text":"Uncaught","exception":{"description":"Error: target is stale"}}}`, "", true, false},
		{"occluded", `{"result":{"type":"undefined"},"exceptionDetails":{"text":"Uncaught","exception":{"description":"Error: target is occluded"}}}`, "", true, false},
		{"other exception", `{"result":{"type":"undefined"},"exceptionDetails":{"text":"Uncaught","exception":{"description":"TypeError: failed"}}}`, "", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			value, err := decodeEvaluateResult([]byte(test.body))
			if (err == nil) != test.ok {
				t.Fatalf("decodeEvaluateResult() error = %v, want success=%v", err, test.ok)
			}
			if errors.Is(err, jev.ErrStaleObservation) != test.stale {
				t.Fatalf("errors.Is(err, ErrStaleObservation) = %v, want %v", errors.Is(err, jev.ErrStaleObservation), test.stale)
			}
			if test.ok && string(value) != test.value {
				t.Fatalf("value = %s, want %s", value, test.value)
			}
		})
	}
}

func TestSelectPageTarget(t *testing.T) {
	targets := []targetInfo{
		{Type: "page", URL: "about:blank", WebSocketDebuggerURL: "ws://blank"},
		{Type: "page", URL: "https://example.test/start", WebSocketDebuggerURL: "ws://requested"},
	}
	if got := selectPageTarget(targets, false, "https://example.test/start"); got != "ws://requested" {
		t.Fatalf("launched target = %q", got)
	}
	if got := selectPageTarget(targets, true, "https://example.test/start"); got != "ws://blank" {
		t.Fatalf("attached target = %q", got)
	}
	if got := selectPageTarget(targets, false, ""); got != "ws://blank" {
		t.Fatalf("default target = %q", got)
	}
}

func TestSelectPageTargetWaitsForRequestedLaunch(t *testing.T) {
	targets := []targetInfo{{Type: "page", URL: "about:blank", WebSocketDebuggerURL: "ws://blank"}}
	if got := selectPageTarget(targets, false, "https://example.test/start"); got != "" {
		t.Fatalf("target = %q, want no target while initial URL is pending", got)
	}
}
