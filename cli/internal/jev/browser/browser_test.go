// Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

package browser

import "testing"

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
