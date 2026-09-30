# Build

The CLI is built with [Bazelisk](https://github.com/bazelbuild/bazelisk) (Bazel launcher); the SDK bridge and native plugins are built with CMake. Bazel fetches all CLI-side dependencies automatically.

Install Bazelisk:

- Windows: `winget install --id Bazel.Bazelisk`
- Linux: install `bazelisk` from your package manager

> [!IMPORTANT]
> Build in this order. In local-SDK mode (the default) the CLI links against the SDK bridge at `sdk/pkg-geniex/` — Bazel expects `sdk/pkg-geniex/lib/geniex.dll` (Windows) or `sdk/pkg-geniex/lib/libgeniex.so` (Linux) to already exist. So build and install the SDK ([Build the SDK](#build-the-sdk)) **first**, then build the CLI ([Build and run the CLI](#build-and-run-the-cli)).

## Windows prerequisites

> [!IMPORTANT]
> Run every command below from **PowerShell** (`pwsh` or Windows PowerShell) — not `cmd.exe`, Git Bash, or WSL. The toolchain step below chains commands through `cmd /c '"...\VsDevCmd.bat" ... && cmake ...'`, and that only parses correctly with PowerShell's quoting (an outer single-quoted string wrapping embedded double-quoted paths). Backslash-escaped quotes (`\"..\"`) are a `cmd.exe` convention — PowerShell doesn't treat `\` as a string escape character, so `cmd /c "...\"C:\Program Files...\"..."` breaks with `'\' is not recognized as an internal or external command`.

### Symlink support (Bazel and CMake)

Needed for both Bazel and the SDK's CMake configure step — a vendored submodule (sentencepiece, under `third-party/geniex-qairt`) creates a symlink during configure and fails with "A required privilege is not held by the client" without this.

1. Enable **Developer Mode**: Settings → Privacy & Security → For developers.
2. Grant **Create symbolic links** rights via `gpedit.msc` → Computer Configuration → Windows Settings → Security Settings → Local Policies → User Rights Assignment, or set `HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System\LocalAccountTokenFilterPolicy = 1` (DWORD).
3. Enable **Long paths**: Settings → Privacy & Security → For developers.
4. **Sign out and back in (or reboot)** after enabling Developer Mode. The toggle showing "On" in Settings isn't enough — already-open sessions keep their old token, which lacks `SeCreateSymbolicLinkPrivilege` until a fresh logon. Verify with `whoami /priv | findstr SymbolicLink` in a new terminal; if it prints nothing, the privilege hasn't taken effect yet.
5. If symlink errors persist, comment out `startup --windows_enable_symlinks` in `.bazelrc` — but be aware this can break other SDK paths.

> [!NOTE]
> **Corporate/domain-joined machines:** on a managed PC, Group Policy can prevent step 4 from ever taking effect — `whoami /priv` keeps showing no `SeCreateSymbolicLinkPrivilege` even after a full reboot with Developer Mode confirmed on (check with `gpresult /r` to confirm GPOs are applied). If that's the case, skip the Developer Mode dance and instead run the build (both `cmake --preset ...` and `bazelisk ...`) from an **elevated** terminal ("Run as Administrator" PowerShell, or an elevated "Developer PowerShell for VS"). Local Administrators get `SeCreateSymbolicLinkPrivilege` by default once elevated, regardless of Developer Mode or domain policy. You can confirm you're a local admin even when GPO is blocking the normal grant via `whoami /groups` — `BUILTIN\Administrators` shows up as "Group used for deny only" in a standard (non-elevated) session, meaning UAC is filtering the privilege rather than it being genuinely absent.

### Toolchain (SDK build)

The SDK's Rust model manager (`sdk/model-manager`) is built by `cargo` from CMake, and its build scripts compile C code, so a native Windows ARM64 build also needs:

- **clang** — C compiler used by Rust build scripts (via `cc-rs`) and the Snapdragon presets
- **cargo** — install Rust with [rustup](https://rustup.rs)
- **Rust target** for Windows ARM64:

  ```powershell
  rustup target add aarch64-pc-windows-msvc
  ```

> [!IMPORTANT]
> `clang` compiles against the MSVC ABI (`--target=arm64-pc-windows-msvc`), so it needs MSVC's `INCLUDE`/`LIB` (e.g. `oldnames.lib`, `msvcrtd.lib`) and `lld-link`/`link.exe` on `PATH`. A plain terminal doesn't have these set, and CMake's compiler-ABI check fails with `lld-link: error: could not open 'oldnames.lib'`. Configure and build from inside a Visual Studio **Developer** environment for the ARM64 target, e.g.:
>
> ```powershell
> cmd /c '"C:\Program Files\Microsoft Visual Studio\<edition>\Common7\Tools\VsDevCmd.bat" -arch=arm64 -host_arch=x64 && cmake --preset arm64-windows-snapdragon-release -B build'
> ```
>
> (adjust the install path/edition for your Visual Studio install; requires the "Desktop development with C++" workload with ARM64 build tools.)

### Native SDKs (for full Snapdragon build)

The `arm64-windows-snapdragon-release` preset requires:

- **Hexagon SDK** — `HEXAGON_SDK_ROOT`, `HEXAGON_TOOLS_ROOT`
- **OpenCL SDK** — `OPENCL_SDK_ROOT` (headers + `OpenCL.lib`; runtime ICD ships with the Snapdragon GPU driver)
- **Windows Driver Kit** — provides `inf2cat.exe`
- **Self-signed HTP cert** (`.pfx`) and Windows test-signing enabled — see [run.md § Self-signed fallback](run.md#self-signed-fallback) for cert generation and test-signing setup

## Build the SDK

### Windows ARM64 (Snapdragon)

> [!NOTE]
> The Hexagon toolchain has a 250-character path limit. Shorten the source path with `subst` before building:
>
> ```powershell
> subst G: C:\path\to\geniex
> cd G:\sdk
> ```

Run from inside a Visual Studio Developer environment for ARM64 (see [Toolchain § clang/MSVC note](#toolchain-sdk-build) above) — either launch a "Developer PowerShell for VS" (ARM64 variant) from the Start menu, or call `VsDevCmd.bat` first. On a corporate/domain-joined machine where the symlink privilege won't take effect (see [Symlink support](#symlink-support-bazel-and-cmake) above), launch that PowerShell **elevated** ("Run as Administrator"):

```powershell
cd sdk
cmake --preset arm64-windows-snapdragon-release -B build
cmake --build build -j
cmake --install build --prefix pkg-geniex
```

Or in one line from a plain (non-Developer) PowerShell, chaining through `VsDevCmd.bat` — note the outer single-quotes (see the PowerShell quoting warning under [Windows prerequisites](#windows-prerequisites)):

```powershell
cmd /c '"C:\Program Files\Microsoft Visual Studio\<edition>\Common7\Tools\VsDevCmd.bat" -arch=arm64 -host_arch=x64 && cmake --preset arm64-windows-snapdragon-release -B build && cmake --build build -j && cmake --install build --prefix pkg-geniex'
```

### Linux (cross-compile from x86_64)

Build the SDK inside the derived Snapdragon Linux toolchain container — it extends [ghcr.io/snapdragon-toolchain/arm64-linux](https://github.com/ggml-org/llama.cpp/blob/master/docs/backend/snapdragon/linux.md#snapdragon-based-linux-devices) with `build-essential`, `ccache`, `rustup`, the `aarch64-unknown-linux-gnu` Rust target, and the cc-rs cross-compiler symlinks baked in (see [`.github/docker/toolchain-linux.Dockerfile`](../.github/docker/toolchain-linux.Dockerfile)). Run from the repo root.

One-shot:

```bash
docker run --rm -u $(id -u):$(id -g) \
    --volume $(pwd):/workspace \
    --workdir /workspace/sdk \
    -e CCACHE_DIR=/workspace/.ccache \
    --platform linux/amd64 \
    docker.io/qualcomm/geniex-toolchain-linux:v0.1.0 \
    bash -c 'cmake --preset arm64-linux-snapdragon-debug -B build-linux . \
      && cmake --build build-linux -j \
      && cmake --install build-linux --prefix pkg-geniex'
```

Interactive:

```bash
docker run --rm -it -u $(id -u):$(id -g) \
    --volume $(pwd):/workspace \
    --workdir /workspace/sdk \
    -e CCACHE_DIR=/workspace/.ccache \
    --platform linux/amd64 \
    docker.io/qualcomm/geniex-toolchain-linux:v0.1.0 bash
# then, inside the container:
cmake --preset arm64-linux-snapdragon-debug -B build-linux .
cmake --build build-linux -j
cmake --install build-linux --prefix pkg-geniex
```

#### CPU-only variant (baseline armv8.0-a)

NPU-less Dragonwing IoT boards (unoq) are baseline ARMv8.0 and trap on the LSE
atomics the `snapdragon` presets inline (see
[#1217](https://github.com/qualcomm/GenieX/issues/1217)). Swap the preset for
`arm64-linux-cpu-{debug,release}` — same container, plain `-march=armv8-a`, and
CPU-only (no QAIRT, Hexagon, or OpenCL):

```bash
cmake --preset arm64-linux-cpu-debug -B build-linux-cpu .
cmake --build build-linux-cpu -j
cmake --install build-linux-cpu --prefix pkg-geniex
```

### Android (cross-compile from Linux)

Build the SDK inside the derived Snapdragon Android toolchain container — it extends [ghcr.io/snapdragon-toolchain/arm64-android](https://github.com/ggml-org/llama.cpp/blob/master/docs/backend/snapdragon/README.md#android) with `build-essential`, `ccache`, `rustup`, and the `aarch64-linux-android` Rust target baked in (see [`.github/docker/toolchain-android.Dockerfile`](../.github/docker/toolchain-android.Dockerfile)). Run from the repo root.

One-shot:

```bash
docker run --rm -u $(id -u):$(id -g) \
    --volume $(pwd):/workspace \
    --workdir /workspace/sdk \
    -e CCACHE_DIR=/workspace/.ccache \
    --platform linux/amd64 \
    docker.io/qualcomm/geniex-toolchain-android:v0.1.0 \
    bash -c 'cmake --preset arm64-android-snapdragon-debug -B build-android . \
      && cmake --build build-android -j \
      && cmake --install build-android --prefix pkg-geniex'
```

Interactive:

```bash
docker run --rm -it -u $(id -u):$(id -g) \
    --volume $(pwd):/workspace \
    --workdir /workspace/sdk \
    -e CCACHE_DIR=/workspace/.ccache \
    --platform linux/amd64 \
    docker.io/qualcomm/geniex-toolchain-android:v0.1.0 bash
# then, inside the container:
cmake --preset arm64-android-snapdragon-debug -B build-android .
cmake --build build-android -j
cmake --install build-android --prefix pkg-geniex
```

#### CPU-only variant (baseline armv8.0-a)

`minSdk` is 27, so the AAR still targets phones that predate the armv8.7 ISA the
`snapdragon` presets bake in and trap at startup (see
[#1217](https://github.com/qualcomm/GenieX/issues/1217)). Swap the preset for
`arm64-android-cpu-{debug,release}` — same container, plain `-march=armv8-a`, and
CPU-only (no QAIRT, Hexagon, or OpenCL):

```bash
cmake --preset arm64-android-cpu-debug -B build-android-cpu .
cmake --build build-android-cpu -j
cmake --install build-android-cpu --prefix pkg-geniex
```

`bindings/android` needs no changes: `assembleRelease` packages whatever
`sdk/pkg-geniex/lib/` holds, so the same gradle project yields the CPU-only AAR
that CI publishes as `geniex-android-aar-cpu-<tag>.aar`.

Deploy and smoke-test on device:

```bash
adb push pkg-geniex /data/local/tmp/geniex
adb push Qwen3-0.6B-Q4_0.gguf /data/local/tmp/geniex/modelfiles/llama_cpp/
adb shell "cd /data/local/tmp/geniex && \
  LD_LIBRARY_PATH=./lib:./lib/llama_cpp \
  GENIEX_PLUGIN_PATH=./lib \
  ./bin/geniex_test_llm"
```

The Android demo app is no longer hosted in this repo — it lives in [`qualcomm/ai-hub-apps`](https://github.com/qualcomm/ai-hub-apps/tree/main/apps/geniex_chat_android). Build the AAR here, then point the demo app at it.

### Building against a different QAIRT SDK's headers

The `qairt` plugin (`sdk/plugins/qairt`) compiles against the QNN C API headers
vendored in `third-party/geniex-qairt/qnn-api/include/` — deliberately the
lowest version it supports, so the compiled plugin accepts the widest range of
runtimes. To compile against a different header set instead (e.g. a
workbench/internal QAIRT checkout), pass `-DQAIRT_QNN_HEADERS=...` through to
any of the presets above — it's a CMake cache variable defined in the
submodule, so it flows straight through `add_subdirectory`:

```bash
cmake --preset arm64-windows-snapdragon-release -B build -DQAIRT_QNN_HEADERS=C:\path\to\qairt\include
```

See [`third-party/geniex-qairt` § Using a different QAIRT runtime](https://github.com/qualcomm/geniex-qairt-plugin#using-a-different-qairt-runtime)
for the expected directory shape and caveats (this only narrows the accepted
runtime range, never widens it). Requires the submodule to be at or past
`geniex-qairt-plugin` commit `9f852a2` (allow external QNN SDK headers).

## Build and run the CLI

With the SDK built and installed into `sdk/pkg-geniex/`, build and run the CLI. Quick smoke test:

```bash
bazelisk run //cli -- infer Qwen/Qwen3-0.6B-GGUF
```

> `//cli` is a convenience alias for `//cli/cmd/geniex:geniex`. Both are used interchangeably in these docs.

### Flags

Flags for `bazelisk build` and `bazelisk run`:

| Flag                                    | Meaning                                                                                                     |
| --------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `--//sdk:sdk_type={local,s3,bazel}`     | SDK source. `local` (default) links against `sdk/pkg-geniex`; `s3` and `bazel` are WIP.                     |
| `--config={linux_arm64,windows_arm64}`  | Cross-compile to the target platform (Go toolchain + CGO + `oci_image` base). `sdk/pkg-geniex/` must match. |

### Development and release targets

Development targets:

- `bazelisk run //cli/release/linux:docker` — build and load the Docker image for the Linux release.

Package release artifacts:

| Target                                 | Output                                               |
| -------------------------------------- | ---------------------------------------------------- |
| `bazelisk build //cli:artifact`        | `bazel-bin/cli/artifact.zip`                         |
| `bazelisk build //cli/release/windows` | `bazel-bin/cli/release/windows/geniex-cli-setup.exe` |
| `bazelisk build //cli/release/linux`   | `bazel-bin/cli/release/linux/geniex-cli-docker.tar`  |

Generated executable (for manual invocation): `bazel-bin/cli/cmd/geniex/geniex_/`, with runtime files under `geniex.runfiles/_main`.

### Test coverage

Use `bazelisk coverage` instead of `bazelisk test`:

```bash
bazelisk coverage //... --combined_report=lcov
```

Bazel's default `--instrumentation_filter` only covers packages that own a test target, so every Go package ships at least a placeholder `package_test.go` to keep itself in the denominator. Combined lcov lands at `bazel-out/_coverage/_coverage_report.dat`. Render it with `genhtml bazel-out/_coverage/_coverage_report.dat -o coverage-html` (gitignored), or summarize on the CLI with `lcov --list <report>`.

## Python bindings

See [bindings/python/README.md](../bindings/python/README.md).
