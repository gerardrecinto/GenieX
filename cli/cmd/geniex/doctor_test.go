// Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/qualcomm/GenieX/cli/internal/testutil"
)

func sampleDoctorReport() doctorReport {
	return doctorReport{
		SDKVersion: "1.2.3",
		Runtimes: []doctorRuntime{
			{
				ID:      "llama_cpp",
				Version: "abc123",
				ComputeUnits: []doctorComputeUnit{
					{ID: "HTP0", Name: "Hexagon NPU"},
					{ID: "GPUOpenCL", Name: "Adreno GPU"},
				},
				Resolutions: []doctorResolution{
					{Alias: "npu", DeviceID: "HTP0", NGL: -1},
					{Alias: "hybrid", NGL: -1},
				},
			},
		},
	}
}

func TestPrintDoctorTextReportsHTPAndAliasResolution(t *testing.T) {
	out, _, err := testutil.CaptureOutput(t, func() error {
		return printDoctorText(sampleDoctorReport())
	})
	if err != nil {
		t.Fatalf("printDoctorText: %v", err)
	}
	for _, want := range []string{
		"GenieX SDK Version: 1.2.3",
		"Runtime: llama_cpp (abc123)",
		"HTP0 (Hexagon NPU)",
		"npu -> HTP0 (ngl=-1)",
		"hybrid -> default (ngl=-1)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintDoctorJSONPreservesRuntimeDiagnostics(t *testing.T) {
	raw, _, err := testutil.CaptureOutput(t, func() error {
		return printDoctorJSON(sampleDoctorReport())
	})
	if err != nil {
		t.Fatalf("printDoctorJSON: %v", err)
	}

	var got doctorReport
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal doctor JSON: %v\n%s", err, raw)
	}
	if got.SDKVersion != "1.2.3" || len(got.Runtimes) != 1 {
		t.Fatalf("unexpected report: %+v", got)
	}
	runtime := got.Runtimes[0]
	if runtime.ID != "llama_cpp" || runtime.ComputeUnits[0].ID != "HTP0" {
		t.Errorf("runtime diagnostics lost: %+v", runtime)
	}
	if runtime.Resolutions[0].Alias != "npu" || runtime.Resolutions[0].DeviceID != "HTP0" {
		t.Errorf("resolution diagnostics lost: %+v", runtime.Resolutions)
	}
}
