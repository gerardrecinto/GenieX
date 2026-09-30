# JEV: structured decisions and local browser agent

JEV is GenieX's non-executing structured-decision subsystem and local browser-agent command:

```text
geniex jev <model> <classify|choose|triage|browser>
```

The public Go package [`bindings/go/jev`](../bindings/go/jev) provides strict typed JSON decoding, closed-set classification, closed-set multiple choice, and constrained browser-action candidates. It has no browser, shell, filesystem-management, or side-effect execution API. Hosts own retries, business validation, user confirmation, and every externally visible action.

## Structured decisions

```bash
# Text-only classification accepts an LLM or VLM.
geniex jev <model> classify \
  --label capital_of_france \
  --label capital_of_germany \
  --label unknown \
  --instruction "Choose the label that correctly answers the question." \
  --context "What is the capital of France?"

# Multiple choice returns stable option IDs, not display text.
geniex jev <model> choose \
  --option paris=Paris \
  --option berlin=Berlin \
  --instruction "Select the capital of France." \
  --context "Question: What is the capital of France?"
```

`classify`, `choose`, and `triage` print one locally validated JSON candidate to standard output, for example `{"label":"capital_of_france"}`, `{"selected_ids":["paris"]}`, or a fixed five-field triage record. Add `--confidence` only when advisory model-reported confidence is useful. Add repeatable `--image` for a visual decision; images require a VLM.

### Customer-support triage

`triage` is a single bundled structured decision designed for a controlled latency comparison with Laya's five-question customer-support preset:

```bash
geniex jev <model> triage \
  --message "I was charged twice this month and want a refund" \
  --threads <count> --threads-batch <count> \
  --benchmark 20
```

It emits exactly one record with categorical values:

```json
{"intent":"refund","is_urgent":"no","frustration":"2","refund_requested":"yes","churn_risk":"no"}
```

The `intent` labels match Laya's stable choice IDs. `is_urgent`, `refund_requested`, and `churn_risk` are GenieX `yes`/`no` categories, and `frustration` is a `0`–`3` category. They are intentionally not Laya's raw `noul` probabilities or continuous score values. The comparison measures one request's architectural latency, not answer quality, calibration, model size, or comparable token throughput: Laya uses one non-autoregressive encoder forward pass with zero generated tokens, while GenieX generates a grammar-constrained JSON record.

To compare JEV with ordinary GenieX inference, add `--benchmark <repetitions>` to `classify`, `choose`, or `triage`. GenieX performs one discarded warmup, resets the loaded model between attempts, then writes timing JSON to standard error while preserving the decision JSON on standard output. The timing report includes native SDK generation time/TTFT, token counts and throughput, the median end-to-end JEV decision time, JEV host overhead, standard deviation, a SHA-256 prompt identifier plus prompt-contract version, and the requested tuning values.

`--threads`, `--threads-batch`, `--batch`, and `--ubatch` are opt-in JEV model controls. Each defaults to `0`, retaining the SDK default; provide only non-negative values. Sweep `--threads` and `--threads-batch` on the target hardware with the same model, prompt, compute unit, context size, image inputs, and token limit. Compare at least 20 post-warmup samples using median end-to-end time and decode tokens/s, verify the typed decisions against a representative corpus, and confirm the applied values with `GENIEX_LOG=info` `[Optimise]` logs. Treat the winning values as a workload-specific invocation profile, not a new global default. JEV prompts usually differ from an ad hoc `infer` prompt, so compare token counts and throughput as well as latency. Browser timing is intentionally excluded: screenshot/DOM observation, approval wait, and browser execution are separate host costs, not model inference.

For the Laya side, run the GenieX-owned external runner without adding Laya or Torch as dependencies:

```bash
python benchmarks/bench_laya_triage.py \
  --laya-root C:/Users/kvo/projects/code/laya \
  --fixture benchmarks/jev_laya_triage_fixture.json \
  --model english --device cpu --threads <same-thread-count> --repetitions 20
```

It preloads Laya before timing, discards one warmup, then emits hot `Router.predict` p50/p95/mean/stddev latency and five-decision throughput. Model load, download, process startup, and JSON serialization are excluded. Use the same CPU-only physical-core-oriented thread policy for both engines, record model/runtime revisions, and keep the two JSON reports together; do not subtract their latencies or compare their token rates.

- **Classification** returns exactly one stable label ID from `ClassificationSpec.Labels`.
- **Multiple choice** returns a bounded, duplicate-free list of stable option IDs from `MultipleChoiceSpec.Options`.
- **Typed extraction** uses `DecodeStructured` with a Go destination struct and a host validator.

Use stable IDs for selectable values. Option text and supplied context are passed to the model as untrusted content, not as model instructions.

```go
model := jev.VLMDecider{VLM: vlm, RuntimeID: runtimeID}
result, err := model.Classify(ctx, jev.ClassificationSpec{
    Labels:       []string{"invoice", "receipt", "other"},
    Instructions: "Classify this document.",
    Context:      extractedText,
    ImagePaths:   []string{screenshotPath},
})
// result.Label is validated to be one of the supplied IDs.
```

For llama.cpp, classification and multiple-choice decisions use a small generated GBNF grammar that enumerates only the supplied IDs. Other runtimes use JSON mode. Every result is still strictly decoded and validated locally, so grammar support is defense in depth rather than the trust boundary.

Set `RequestConfidence` to receive a model-reported value from 0 through 1. It is marked `self_reported`; it is not calibrated, and it must never be used to authorize an action, skip confirmation, or bypass a host safety policy.

```go
type extraction struct {
    Name string `json:"name"`
}
var result extraction
err := jev.DecodeStructured(rawModelOutput, &result, func() error {
    if result.Name == "" {
        return errors.New("name is required")
    }
    return nil
})
```

`DecodeStructured` accepts one optional fenced JSON object and rejects prose, arrays, unknown fields, and multiple JSON values. It intentionally does not synthesize a grammar from JSON Schema; use strict typed decoding plus semantic validation for generic structures.

## Browser agent

`browser` is an experimental local browser-agent mode. It uses a vision-language model (VLM) to inspect a fresh Chrome or Edge screenshot and a constrained list of visible page controls, then takes one validated browser action at a time.

```bash
# Starts Chrome or Edge with a fresh temporary browser profile.
geniex jev google/gemma-4-E4B-it-qat-q4_0-gguf browser \
  --task "Find the GenieX documentation and summarize the supported platforms" \
  --url https://github.com/qualcomm/GenieX

# Inspect a task without executing any actions.
geniex jev google/gemma-4-E4B-it-qat-q4_0-gguf browser \
  --task "Find the pricing page" \
  --url https://example.com \
  --dry-run
```

A VLM is required for browser mode. `llama_cpp` VLMs additionally use grammar-constrained output when supported; other runtimes use JSON mode plus strict local validation.

### Browser connection

By default, GenieX finds Chrome or Edge, launches it with DevTools Protocol enabled, and uses an isolated temporary profile. That prevents the agent from automatically inheriting browser cookies and saved credentials.

```bash
# Attach to an existing browser that was launched with remote debugging.
geniex jev <vlm> browser --task "..." --attach http://127.0.0.1:9222

# Launch a specific executable and an explicitly selected profile.
geniex jev <vlm> browser --task "..." \
  --browser-path "C:\Program Files\Google\Chrome\Application\chrome.exe" \
  --profile C:\safe-browser-profile
```

Only give the command a profile or existing browser session that is appropriate for the task. An attached browser can contain authenticated pages and private data.

### Safety boundary

The model never gets arbitrary JavaScript, CSS selectors, shell access, or generic DevTools Protocol access. Each browser observation produces a bounded indexed element map; model actions can only refer to an item in the current map. Targets are checked for visibility and occlusion both when observed and before an event is dispatched. GenieX fingerprints the trusted URL, title, and indexed element map at observation time and rechecks it before every browser action; changed page state is treated as stale and requires a new observation and model decision.

With the default `--auto read` policy, these actions can run automatically when their targets are unambiguously safe:

- read/extract, scroll, wait, history navigation;
- same-origin navigation;
- ordinary non-submitting text entry;
- normal link navigation.

The terminal requires a decision for every uncertain or consequential action, including:

- login, passwords, payment details, and form submission;
- purchases, transfers, sends, subscriptions, publishing, and sharing;
- deletes, removals, clears, and unsubscribe actions;
- downloads, uploads/file choosers, and new windows/tabs;
- cross-origin navigation.

Choose **Approve**, **Skip**, or **Abort** at each prompt. Approvals are not remembered. `--auto none` prompts for every action. `--auto all` disables prompts and prints a warning; it does not bypass action validation, element freshness checks, or the fixed action allowlist. Use it only in a controlled test browser profile.

A classification, multiple-choice result, or advisory confidence value cannot make a login, submission, purchase, deletion, upload, download, new-window action, or cross-origin navigation safe. `cli/internal/jev` retains all browser action validation, freshness checks, redaction, and explicit approval.

`--dry-run` never executes browser actions. `--trace-dir <directory>` preserves screenshots and trusted action summaries for debugging; do not use trace directories to store sensitive task data.

### Limits

- The command has a fixed default of 25 browser actions; change this only with `--max-steps`.
- Each model decision is a single JSON action. Invalid JSON, unknown actions, stale element indices, non-HTTP(S) URLs, and unsafe input shapes are rejected and retried a bounded number of times.
- The agent stops when the model reports completion, the step limit is reached, the task makes no progress, cancellation occurs, or the user aborts approval.
- Browser automation is for authorized local use. Review every approval prompt before accepting it.

## Development notes

The MVP is intentionally built above existing LLM and VLM APIs. It does not change `sdk/include/geniex.h`, backend plugins, or third-party source. The optional `bindings/go/jev.Shortlist` seam lets a host narrow a large trusted candidate set with its own ranker; it cannot add or mutate candidate IDs, is not automatically enabled for browser controls, and final validation remains against the host's full map. A later experimental phase may evaluate SemIf/OpenJev-style direct-logit action scoring through the existing llama.cpp LLM logits API, but VLM logit-derived confidence needs a separately measured and reviewed SDK proposal.
