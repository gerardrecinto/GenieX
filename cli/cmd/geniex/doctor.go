// Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	geniex_sdk "github.com/qualcomm/GenieX/bindings/go"
)

var doctorAliases = []string{
	geniex_sdk.ComputeUnitCPU,
	geniex_sdk.ComputeUnitGPU,
	geniex_sdk.ComputeUnitNPU,
	geniex_sdk.ComputeUnitHybrid,
}

type doctorReport struct {
	SDKVersion       string          `json:"sdk_version"`
	QairtRuntimePath string          `json:"qairt_runtime_path,omitempty"`
	Runtimes         []doctorRuntime `json:"runtimes"`
}

type doctorRuntime struct {
	ID           string              `json:"id"`
	Version      string              `json:"version"`
	ComputeUnits []doctorComputeUnit `json:"compute_units"`
	Resolutions  []doctorResolution  `json:"resolutions"`
}

type doctorComputeUnit struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type doctorResolution struct {
	Alias    string `json:"alias"`
	DeviceID string `json:"device_id,omitempty"`
	NGL      int32  `json:"ngl"`
	Warning  string `json:"warning,omitempty"`
}

// collectDoctorReport queries only SDK and runtime metadata. It does not
// initialize the model manager, resolve a model, or perform inference.
func collectDoctorReport() (doctorReport, error) {
	if err := geniex_sdk.Init(); err != nil {
		return doctorReport{}, fmt.Errorf("initialize SDK: %w", err)
	}
	defer geniex_sdk.DeInit()

	runtimes, err := geniex_sdk.GetRuntimeList()
	if err != nil {
		return doctorReport{}, fmt.Errorf("list runtimes: %w", err)
	}
	sort.Strings(runtimes.RuntimeIDs)

	report := doctorReport{
		SDKVersion:       geniex_sdk.Version(),
		QairtRuntimePath: geniex_sdk.GetQairtRuntimePath(),
		Runtimes:         make([]doctorRuntime, 0, len(runtimes.RuntimeIDs)),
	}
	for _, runtimeID := range runtimes.RuntimeIDs {
		units, err := geniex_sdk.GetComputeUnitList(geniex_sdk.GetComputeUnitListInput{RuntimeID: runtimeID})
		if err != nil {
			return doctorReport{}, fmt.Errorf("list compute units for %s: %w", runtimeID, err)
		}

		runtime := doctorRuntime{
			ID:           runtimeID,
			Version:      geniex_sdk.GetPluginVersion(runtimeID),
			ComputeUnits: make([]doctorComputeUnit, 0, len(units.ComputeUnits)),
			Resolutions:  make([]doctorResolution, 0, len(doctorAliases)),
		}
		for _, unit := range units.ComputeUnits {
			runtime.ComputeUnits = append(runtime.ComputeUnits, doctorComputeUnit{ID: unit.ID, Name: unit.Name})
		}
		for _, alias := range doctorAliases {
			resolved, err := geniex_sdk.ResolveDevice(geniex_sdk.ResolveDeviceInput{
				RuntimeID:   runtimeID,
				ComputeUnit: alias,
				NglDefault:  -1,
			})
			if err != nil {
				return doctorReport{}, fmt.Errorf("resolve %s for %s: %w", alias, runtimeID, err)
			}
			runtime.Resolutions = append(runtime.Resolutions, doctorResolution{
				Alias:    alias,
				DeviceID: resolved.DeviceID,
				NGL:      resolved.Ngl,
				Warning:  resolved.Warning,
			})
		}
		report.Runtimes = append(report.Runtimes, runtime)
	}
	return report, nil
}

func printDoctorText(report doctorReport) error {
	fmt.Printf("GenieX SDK Version: %s\n", report.SDKVersion)
	if report.QairtRuntimePath != "" {
		fmt.Printf("QAIRT Runtime Override: %s\n", report.QairtRuntimePath)
	}
	for _, runtime := range report.Runtimes {
		fmt.Printf("\nRuntime: %s (%s)\n", runtime.ID, runtime.Version)
		fmt.Println("  Compute units:")
		for _, unit := range runtime.ComputeUnits {
			fmt.Printf("    - %s (%s)\n", unit.ID, unit.Name)
		}
		fmt.Println("  Resolved aliases:")
		for _, resolution := range runtime.Resolutions {
			deviceID := resolution.DeviceID
			if deviceID == "" {
				deviceID = "default"
			}
			fmt.Printf("    - %s -> %s (ngl=%d)\n", resolution.Alias, deviceID, resolution.NGL)
			if resolution.Warning != "" {
				fmt.Printf("      warning: %s\n", resolution.Warning)
			}
		}
	}
	return nil
}

func printDoctorJSON(report doctorReport) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func doctor() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		GroupID: "management",
		Use:     "doctor",
		Short:   "Inspect local runtime and accelerator readiness",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := collectDoctorReport()
			if err != nil {
				return err
			}
			if jsonOutput {
				return printDoctorJSON(report)
			}
			return printDoctorText(report)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output diagnostics as JSON")
	return cmd
}
