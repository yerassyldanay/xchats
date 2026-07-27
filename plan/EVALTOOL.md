# Evaluation Tool Architecture

## Status

This document is a design and implementation handoff. It describes the recommended
evaluation platform for xchats, why it is shaped this way, how it should coexist with the
current evaluation code, and what another AI agent or engineer should implement.

The central recommendation is:

> Use a standalone Go evaluation application for test execution and grading, and use a
> dedicated Langfuse project as the dataset store, prompt registry, experiment history,
> trace viewer, score database, comparison UI, and agent-accessible API.

Promptfoo may remain as a runner adapter during migration, but it should not remain the
authoritative results UI. Local JSON artifacts must also remain available so evaluation
history is portable and not dependent on any vendor or retention policy.

## Why this is the recommended design

The evaluation requirements extend beyond sending a text prompt to several models. The
system must evaluate full pipelines: WhatsApp response generation against a knowledge
base, image and document extraction, audio transcription, video preprocessing, draft
generation from extracted media, and database-shaped structured output. These pipelines
need custom preprocessing and deterministic contract checks that are specific to xchats.
No general-purpose evaluation product can own that logic without duplicating application
rules in tool-specific configuration or scripts.

Promptfoo is a useful prompt-matrix runner and provides a convenient assertion DSL. It is
well suited to inputs and outputs that can be expressed directly in a Promptfoo config.
It is less suitable as the long-term system of record for multi-stage media pipelines,
intermediate artifacts, production traces, prompt lifecycle, and agent-driven analysis.
The existing extraction evaluator already had to bypass Promptfoo to support image
downscaling, OpenRouter-specific request parameters, retry behavior, and Go grading.

Langfuse already exists in this repository's operational architecture. The backend emits
OpenTelemetry traces to Langfuse, including model metadata, token usage, sessions, inputs,
outputs, and nested generation observations. Langfuse also provides datasets, versioned
prompts, experiment runs, media attachments, scores, comparison views, a public API, a
CLI, and an MCP server. It is therefore a better common UI and result database than adding
a second platform.

Langfuse is not a replacement for the evaluation runner. It should receive the structured
record of what the runner did and how each result was graded. The Go runner remains the
source of truth for deterministic requirements; Langfuse makes those requirements and
results inspectable, comparable, and accessible to people and AI agents.

## Goals

The platform must support:

1. Multiple evaluation intentions, including text, media, schema, and end-to-end tests.
2. A matrix of prompt versions, models, model parameters, preprocessors, response schemas,
   and pipeline implementations.
3. Declarative test cases with expected outputs and reusable requirement types.
4. Deterministic Go graders, optional LLM judges, and optional human review.
5. Source and processed media, intermediate artifacts, raw outputs, parsed outputs,
   retries, errors, latency, tokens, and estimated cost.
6. Stable experiment history and side-by-side comparison against a baseline.
7. An isolated evaluation environment that can evolve separately from the backend.
8. A way to test the real backend when production parity matters.
9. Machine-readable access for Codex, Claude, CI jobs, and other agents.
10. Safe agent iteration on candidate prompts or code without direct production changes.
11. Local, portable results even when Langfuse is unavailable or old hosted data expires.

## Non-goals

The first implementation should not:

- Build a new custom web dashboard.
- Move all production prompts into Langfuse immediately.
- Rewrite every existing Promptfoo suite in one change.
- Use an LLM judge for requirements that can be checked deterministically.
- Allow an evaluation agent to modify production prompt labels or deploy backend code.
- Make the evaluation module depend directly on `backend/internal` packages.

## High-level architecture

```text
                         Dedicated Langfuse project
                        ┌──────────────────────────┐
evals/suites/*.yaml ───▶│ Datasets and versions    │
evals/prompts/* ───────▶│ Prompt versions          │
                        └────────────┬─────────────┘
                                     │
                                     ▼
                         Standalone Go eval runner
                        ┌──────────────────────────┐
                        │ Load case                │
                        │ Resolve variant          │
                        │ Preprocess input         │
                        │ Run model/pipeline       │
                        │ Parse output             │
                        │ Run graders              │
                        └────────────┬─────────────┘
                                     │
                    ┌────────────────┼────────────────┐
                    ▼                ▼                ▼
               Local JSON       Langfuse trace   Langfuse scores
               and artifacts    and run item     and comments
                    │                │                │
                    └────────────────┼────────────────┘
                                     ▼
                             Langfuse experiment UI
                         compare / filter / inspect / review
                                     │
                                     ▼
                         Codex / Claude / CI via API,
                              CLI, MCP, or evalctl
```

## Repository shape

The existing `evals/harness` module should evolve rather than be discarded. A possible
target layout is:

```text
evals/
├── cmd/
│   └── evalctl/
├── internal/
│   ├── artifacts/
│   ├── cases/
│   ├── datasets/
│   ├── graders/
│   ├── langfuse/
│   ├── media/
│   ├── models/
│   ├── prompts/
│   ├── runners/
│   └── variants/
├── suites/
│   ├── text/
│   ├── media/
│   ├── pipeline/
│   └── schema/
├── prompts/
├── fixtures/
├── artifacts/              # generated, ignored by Git
├── runs/                   # local durable result manifests
└── models.yaml
```

The exact migration can be incremental. Existing packages and commands do not need to be
moved before the abstractions are proven.

## Suite taxonomy

Use one dedicated Langfuse project, for example `xchats-evals`. Organize evaluation
intentions as separate datasets. Suggested names:

```text
text/whatsapp-response-v1
text/language-quality-v1

media/image-extraction-v1
media/document-extraction-v1
media/audio-transcription-v1
media/video-understanding-v1

pipeline/draft-generation-v1
pipeline/end-to-end-v1

schema/kb-draft-v1
schema/database-import-v1
schema/structured-output-v1
```

Do not put unrelated intentions into one dataset merely to obtain a single table. The
Langfuse project is the common home; datasets provide meaningful comparison boundaries.
Runs are comparable when they execute the same dataset or a deliberately selected version
of it.

## Test case model

Test definitions stay in Git as the reviewable source of truth. They are synchronized to
Langfuse datasets so Langfuse can align cases across experiment runs.

Example image extraction case:

```yaml
id: pricing-card-001

input:
  type: image
  file: ../../assets/fixed_tariff_006.png

expected:
  schema: extraction-v1
  fields:
    content_kind: infographic
    visibility_suggestion: visible
    media_role_hint: pricing
  text_contains_all:
    - "Kaspi Pay"
    - "25 000"
  allowed_numbers:
    - "3"
    - "25 000"

metadata:
  intent: media-parsing
  modality: image
  language: ru
  difficulty: medium
```

Example WhatsApp response case:

```yaml
id: price-and-delivery-ru-001

input:
  message: "Сколько стоит кофемашина и сколько будет доставка?"
  history: []
  knowledge_base_fixture: ../../fixtures/kb/shop-v3.json

expected:
  response_schema: whatsapp-draft-v2
  requires:
    - price-token
    - delivery-cost-token
  language: ru
  escalate: false
  forbid_invented_numbers: true
  allowed_media_groups: []

metadata:
  intent: whatsapp-response
  category: price-and-delivery
```

Stable case IDs are mandatory. Dataset synchronization should be idempotent and preserve
case identity across updates. Because Langfuse dataset item IDs are project-scoped, derive
them from both suite name and case ID or persist the assigned IDs in a local manifest.

## Variant model

An experiment variant is more than a model name. Define it explicitly:

```go
type Variant struct {
	Model               string
	ModelConfig         map[string]any
	PromptName          string
	PromptVersion       string
	PreprocessorVersion string
	PipelineVersion     string
	ResponseSchema      string
	KnowledgeBaseVersion string
}
```

Every experiment run must record at least:

```text
eval_suite
dataset_version
model
model_parameters
prompt_name
prompt_version
preprocessor_version
pipeline_version
response_schema_version
knowledge_base_version
runner_git_sha
environment=evaluation
```

Prefer one stable variant per experiment run. This produces useful comparisons such as:

```text
Gemini + prompt v3 + original image
Gemini + prompt v4 + original image
Gemini + prompt v4 + resize-2048
GPT-4o-mini + prompt v4 + resize-2048
```

Run names should be human-readable, but filtering and reproducibility must depend on
metadata rather than parsing a name string.

## Runner abstraction

The evaluation core should not care how a result is produced:

```go
type Runner interface {
	Run(context.Context, Case, Variant) (Execution, error)
}
```

Initial runner implementations may include:

- `OpenRouterRunner`: makes direct API calls and supports all custom request behavior.
- `PromptfooRunner`: runs existing Promptfoo suites during migration and converts the raw
  result into the common `Execution` model.
- `FixtureRunner`: replays stored outputs without making paid model calls.
- `ReferenceRunner`: executes experimental code owned entirely by the eval module.
- `BackendRunner`: invokes the actual backend through a public boundary when production
  parity must be tested.

This staged design allows existing Promptfoo suites to continue working while Langfuse
becomes the common result UI. Promptfoo can later be removed if it no longer provides
unique value.

## Evaluation families

### WhatsApp response against a knowledge base

Each case contains the customer message, conversation history, a frozen knowledge-base
snapshot, and expected behavior. The runner constructs the same logical prompt supplied to
the model. Variants may change the system prompt, user prompt framing, model, temperature,
knowledge-base representation, response schema, and post-processing implementation.

Recommended deterministic scores include:

```text
contract/parse-ok
contract/required-fields
contract/known-token-references
contract/known-media-references
contract/no-leftover-placeholders
ground-truth/requires
ground-truth/media-selection
ground-truth/escalation
ground-truth/language
ground-truth/no-invented-numbers
quality/overall-pass
```

Naturalness, tone, or whether the reply is a sensible next step may use a separately named
LLM-judge score or human annotation. Never hide subjective and deterministic checks behind
one unexplained score.

### Information retrieval from media files

Each case contains the source media and ground-truth requirements. The trace should show
the preprocessing stages, model request, raw response, parsed response, retries, and
grading.

For images, test original versus scaled or reformatted inputs. For documents, compare
native PDF input, extracted text, and rendered-page strategies. For audio, compare native
audio models, normalized audio, chunking, and transcription-first approaches. Langfuse
currently documents inline support for common image formats, MP3/WAV/MPEG audio, PDF, and
plain-text attachments.

Raw video is a known limitation. Until Langfuse documents first-class video rendering,
represent a video experiment using:

```text
original video URL or artifact reference
selected frame images
extracted audio
transcript
frame-selection manifest
```

The runner can still evaluate native video-capable models; only the UI representation of
the original binary needs this workaround.

### Draft generation from a draft version, knowledge base, and media content

This suite tests the second stage independently from media extraction. A case contains a
draft schema version, knowledge-base snapshot, previously extracted media result, customer
message, and optional history. Stored extraction fixtures make this stage repeatable and
avoid paying for the vision or transcription model on every run.

Variants may change the prompt, model, draft schema, knowledge serialization, media
metadata representation, and injection logic. Graders should verify schema correctness,
fact provenance, token resolution, media resolution, escalation, absence of invented
values, and the final injected customer-facing reply.

### Database-shaped or structured responses

Each case declares the target schema version and its semantic constraints. Validate model
outputs with standard JSON Schema or a SQL parser, then apply xchats-specific checks:

```text
required fields and types
known tables and columns
allowed enum values
referential integrity
uniqueness constraints
source provenance
no invented records
compatibility with the target migration/schema version
```

Store both the raw response and the parsed representation in the trace. Schema validation
errors become score comments so a human or agent can diagnose the exact failure without
rerunning the model.

## Requirements and graders

Promptfoo provides a built-in assertion DSL. Replacing Promptfoo does not mean rewriting
every assertion by hand for every case. Requirements remain declarative in suite files,
and each requirement type is implemented once as a reusable Go grader.

```go
type Grader interface {
	Grade(context.Context, Case, Execution) ([]Score, error)
}

type Score struct {
	Name    string
	Type    ScoreType
	Value   any
	Comment string
	Metadata map[string]any
}
```

Initial reusable requirement types should include:

```text
equals / one-of
contains-all / contains-any / must-not-contain
JSON parse and JSON Schema validation
allowed and forbidden numbers
known token/media references
required/forbidden fields
language
escalation
SQL/schema validation
latency/token/cost thresholds
```

Publish each meaningful check as its own Langfuse score. Use stable names across suites so
metrics remain comparable. Add `overall_pass` and `checks_pass_rate`, but do not discard
the component scores.

## Langfuse mapping

Use a dedicated Langfuse project rather than mixing synthetic eval traffic with production
analytics.

| Eval concept | Langfuse object |
|---|---|
| Suite | Dataset |
| Case | Dataset item |
| Expected requirements | Dataset item expected output and metadata |
| Variant execution across a suite | Dataset run / experiment |
| One case result | Dataset run item linked to a trace |
| Full pipeline execution | Trace |
| Model call | Generation observation |
| Preprocess/parse/grade step | Span observation |
| Requirement result | Score attached to trace or run |
| Source/processed file | Media attachment or external artifact reference |
| Prompt candidate | Prompt version and candidate label |

Langfuse-hosted datasets are required for the complete run overview and comparison
experience. Purely local datasets currently create traces but not equivalent dataset runs.
The Git suite files remain canonical, but they must be synchronized before a comparable
experiment is run.

## Result sinks

The runner must support multiple output sinks:

```go
type Sink interface {
	StartRun(context.Context, Run) error
	Record(context.Context, ExecutionResult) error
	FinishRun(context.Context, RunSummary) error
}
```

Implement at least:

- `LocalSink`: writes a run manifest, per-case JSON, and referenced artifacts. This is the
  durable, portable record and offline debugging path.
- `LangfuseSink`: synchronizes datasets/media, emits traces, links run items, and publishes
  scores.

Sink failures must not silently lose a paid model result. Write the local result first,
then upload it. Persist enough Langfuse IDs and idempotency keys to retry a partial upload
without rerunning the model.

## Prompt ownership and versioning

Two models are acceptable:

1. Git-managed prompts under `evals/prompts`, with name, content hash, and Git SHA recorded
   on every run.
2. Langfuse-managed prompts, using immutable versions and labels.

The recommended initial model is hybrid:

- Git is the canonical reviewed source.
- Prompts are synchronized into Langfuse for UI experiments and diffs.
- Every execution records both the Git SHA and Langfuse prompt version.
- Candidate labels use a namespace such as `candidate/codex-<run-id>`.
- Production labels are never changed by the evaluation runner.

The production backend does not have to adopt Langfuse-managed prompts as part of this
project. That migration is a separate decision.

## Isolation and backend parity

The evaluation application should remain a separate Go module with its own dependencies,
commands, fixtures, and release cadence. It may implement experimental pipelines that do
not yet exist in the backend.

Isolation creates a drift risk: a successful reference implementation does not prove that
the shipping backend behaves the same way. Address this explicitly through runner adapters
and contract tests. The eval system should be able to run:

- an eval-only reference implementation for research;
- the real backend through an HTTP/CLI boundary for release confidence;
- stored fixtures for cheap and deterministic regression tests.

Do not import `backend/internal` packages from the separate module. If both systems need a
wire contract, publish a small versioned shared package or communicate through serialized
requests and responses.

## Agent access and correction loop

Langfuse exposes experiments, experiment items, datasets, traces, observations, scores,
metrics, comments, and prompts through its public API. The Langfuse CLI wraps the API and
can return machine-readable JSON. Its MCP server exposes experiment and prompt operations
for agents that cannot use a local CLI.

Provide a higher-level `evalctl` interface as well:

```bash
evalctl datasets sync
evalctl run --suite media/image-extraction-v1 --variant variants/gemini-v4.yaml
evalctl runs list --json
evalctl runs inspect --run <id> --failures --json
evalctl runs compare --baseline <id> --candidate <id> --json
evalctl prompts create-candidate --name media/extract --from-version 4
```

The intended agent loop is:

```text
1. Query a recent experiment and its scores.
2. Select failed or regressed cases.
3. Fetch traces, outputs, expected outputs, and score comments.
4. Form a concrete hypothesis.
5. Edit a Git prompt/config on a branch, or create a candidate prompt version.
6. Run the isolated experiment.
7. Compare the candidate against a fixed baseline.
8. Apply policy gates in CI.
9. Request human approval.
10. Promote through the normal deployment process.
```

Agents must use credentials scoped to `xchats-evals`. Do not give evaluation agents the
production Langfuse project key or permission to deploy code. If the Langfuse tier does not
support protected prompt labels, project separation is the primary safety boundary.

## Evaluation policy

Add a version-controlled policy file for release and agent gates:

```yaml
gates:
  required_scores:
    contract/parse-ok: 1.0
    contract/known-token-references: 1.0
    ground-truth/no-invented-numbers: 1.0

  maximum_regression:
    quality/overall-pass: 0.0

  informational_scores:
    - quality/naturalness
    - performance/cost-usd
    - performance/latency-ms

promotion:
  require_human_approval: true
  agents_may_update_production_label: false
```

The policy evaluator should read machine-readable experiment results from Langfuse or the
local manifest and return a non-zero exit code when a gate fails.

## Suggested commands

The final CLI should support workflows like:

```bash
# Synchronize Git suites into Langfuse datasets.
evalctl datasets sync

# Run one suite with its default matrix.
evalctl run --suite text/whatsapp-response-v1

# Run a targeted matrix.
evalctl run \
  --suite media/image-extraction-v1 \
  --models google/gemini-2.5-flash,openai/gpt-4o-mini \
  --prompts 3,4 \
  --preprocessors original,resize-2048

# Replay parsing and grading without paid calls.
evalctl run --suite schema/database-import-v1 --runner fixture

# Compare and gate a candidate.
evalctl runs compare --baseline main --candidate current --json
evalctl gate --baseline main --candidate current
```

## Migration plan

### Phase 1: Common result model and local sink

1. Introduce `Case`, `Variant`, `Execution`, `Score`, `Runner`, `Grader`, and `Sink` types.
2. Adapt the current extraction evaluator to these types without changing its behavior.
3. Implement `LocalSink` and preserve the existing JSON/Markdown reports.
4. Add fixture-based tests for parsing, grading, and sink recovery.

### Phase 2: Langfuse dataset and result sink

1. Create a dedicated `xchats-evals` Langfuse project and project-scoped credentials.
2. Implement dataset synchronization with stable case identity and content hashes.
3. Reuse or reproduce the repository's OTLP/HTTP setup in the eval module.
4. Emit one trace per case/variant and generation observations for model calls.
5. Implement media upload through the Langfuse media API.
6. Link traces to dataset run items.
7. Publish all component scores and overall summaries.
8. Verify side-by-side comparison in the Langfuse UI.

### Phase 3: Migrate existing suites

1. Migrate image extraction first because it already has deterministic cases and checks.
2. Migrate the existing Promptfoo text scenarios through `PromptfooRunner`, publishing
   their Go verdicts into Langfuse.
3. Replace Promptfoo execution with direct runners only where that reduces complexity.
4. Add document, audio, video decomposition, draft-generation, and schema suites.

### Phase 4: Agent and CI workflows

1. Add machine-readable `list`, `inspect`, `compare`, and `gate` commands.
2. Configure the Langfuse CLI or MCP server for evaluation agents.
3. Implement candidate prompt naming and labels.
4. Add CI baseline selection and regression gates.
5. Require human approval for production promotion.

### Phase 5: Optional UI-triggered experiments

Langfuse supports a remote experiment webhook on a dataset. After the CLI workflow is
stable, expose a small service that accepts the Langfuse trigger, runs the external Go
pipeline, and publishes the experiment. Do not make this service the first milestone.

## Acceptance criteria

The first production-worthy milestone is complete when:

1. `evalctl run` executes at least one existing extraction suite against two models.
2. The same source cases exist as a versioned Langfuse dataset.
3. Each case/model execution appears as a dataset run item with a linked trace.
4. The trace displays the source image, processed image, prompt, raw output, parsed output,
   model, token usage, retries, latency, and cost basis.
5. Every existing Go extraction check appears as an individual Langfuse score with failure
   details.
6. Overall pass rate is visible for the run.
7. Two variants can be compared side by side in Langfuse using the same dataset version.
8. The complete result also exists locally and can be re-uploaded without a model call.
9. An agent can list runs, retrieve failed items and scores, and follow them to traces using
   the API, CLI, MCP, or `evalctl`.
10. CI can fail on a deterministic contract regression.
11. No evaluation credential grants access to modify production prompts or deployment.

## Known limitations and risks

- Langfuse does not currently provide a high-level Go experiment runner; REST API and
  OpenTelemetry integration must be implemented in the eval module.
- Raw video preview is not a documented first-class Langfuse media format.
- Hosted-plan retention can delete old traces while dataset run references remain. Local
  artifacts or self-hosting are required for durable history.
- Media uploads require deliberate PII and confidentiality rules. Production's current
  decision to omit raw customer images must not be weakened accidentally.
- Self-hosted media requires correctly configured S3-compatible object storage.
- Model output remains nondeterministic; critical comparisons may require repeated trials.
- LLM judges require calibration and must not replace deterministic safety checks.
- An isolated reference runner may drift from the backend unless parity runs are maintained.
- API schemas change over time. Generate or verify API calls against the current Langfuse
  OpenAPI specification rather than relying on stale handwritten request types.

## Open implementation decisions

The implementer should resolve these before Phase 2 is merged:

1. Whether prompt text remains exclusively Git-managed or is synchronized to Langfuse.
2. Whether Langfuse datasets or Git suite files are authoritative for human edits. This
   document recommends Git as authoritative.
3. Whether local run artifacts are committed, stored externally, or retained only in CI.
4. How baseline runs are named and selected per branch and suite.
5. Which media may be uploaded to hosted Langfuse versus represented by redacted metadata.
6. Whether the first Langfuse sink uses a small handwritten REST client or a generated
   client from the official OpenAPI schema.
7. How the backend runner is invoked without importing `backend/internal` code.

## Official references

- Langfuse evaluation concepts:
  https://langfuse.com/docs/evaluation/core-concepts
- Langfuse experiment runner:
  https://langfuse.com/docs/evaluation/experiments/experiments-via-sdk
- Langfuse experiment data model:
  https://langfuse.com/docs/evaluation/experiments/data-model
- Langfuse datasets:
  https://langfuse.com/docs/evaluation/experiments/datasets
- Langfuse scores:
  https://langfuse.com/docs/evaluation/scores/overview
- Scores submitted by external pipelines:
  https://langfuse.com/docs/evaluation/evaluation-methods/scores-via-sdk
- Langfuse media and attachments:
  https://langfuse.com/docs/observability/features/multi-modality
- Prompt version control:
  https://langfuse.com/docs/prompt-management/features/prompt-version-control
- Public API:
  https://langfuse.com/docs/api-and-data-platform/features/public-api
- CLI:
  https://langfuse.com/docs/api-and-data-platform/features/cli
- MCP prompt management:
  https://langfuse.com/docs/prompt-management/features/mcp-server
- Experiment query API and MCP announcement:
  https://langfuse.com/changelog/2026-07-07-experiments-public-api-and-mcp

