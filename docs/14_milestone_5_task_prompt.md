# Milestone 5 Task Prompt - Real Provider Adapters and Credential Broker v1

Implement only Milestone 5. Milestones 0 through 4 are complete and are
regression constraints. Do not add imports, extraction, embeddings, RAG,
timeline-aware context, chat, branches, streaming UI, model discovery, or an OS
keyring integration.

This document is the durable implementation contract. When a general project
document is less specific, this document controls Milestone 5 behavior. Follow
the ordered red/green/refactor sequence in
`.plans/milestone_5_implementation.md`.

## Outcome

An author can configure non-secret, application-level provider profiles for an
OpenAI-compatible chat endpoint or an Ollama chat endpoint, see whether each
profile has the required environment credential, and run the existing Line
Polish action with a compatible real-provider style. The action still produces
a transient reviewable patch. No provider response changes canon until the
author explicitly accepts it through the Milestone 4 acceptance path.

At least one local no-auth endpoint path and one bearer-API-key path must execute
through the same provider-neutral generation interface. Provider credentials
must never be stored in, copied into, returned from, or logged with a story
project.

## Starting state and design constraints

Milestone 4 provides:

- strict project-local agent and style registries,
- pure applicability and minimal-context assembly,
- a provider-neutral `TextGenerator` boundary,
- a deterministic mock provider,
- transient action runs,
- explicit patch accept/reject with revision, rollback, index, and Git safety,
- an editor action menu and inline patch review.

Preserve those guarantees. Extend the provider selection behind the generation
boundary; do not fork the action lifecycle by provider type. In particular:

- provider profiles are application configuration, not story canon,
- project-local styles may reference provider profile IDs but contain no secret,
- the browser never sends or receives provider keys,
- an unavailable or incompatible provider removes a style from action
  availability,
- run-time validation repeats compatibility and credential checks,
- run and reject remain zero-mutation operations,
- accept remains the only route by which generated text may become canon.

## Scope boundaries

In scope:

- one strict application-level `providers.yaml` document,
- atomic, revision-protected replacement of that document through API and UI,
- environment-variable credential lookup through an injected broker,
- OpenAI-compatible `/chat/completions` request/response mapping,
- Ollama `/api/chat` request/response mapping,
- bearer authentication for OpenAI-compatible profiles,
- no-auth local profiles for OpenAI-compatible and Ollama endpoints,
- provider capability declarations and pure agent/style/profile compatibility,
- real-provider styles in the existing project-local style registry,
- capability- and credential-aware action availability,
- real-provider execution in the existing selection patch workflow,
- strict outbound URL, redirect, timeout, body-size, and error handling,
- provider settings UI and status states,
- mock-provider compatibility for existing Milestone 4 projects and tests.

Out of scope:

- accepting or returning an API key through any Storywork HTTP route,
- writing credentials to disk, browser storage, SQLite, story files, or logs,
- OS keychain/keyring support,
- OAuth, device flows, refresh tokens, or credential rotation,
- provider/model auto-discovery or capability probing,
- arbitrary custom headers, query parameters, or URL templates,
- streaming, tool calls, structured output, images, audio, embeddings, or RAG,
- retries, fallback profiles, load balancing, or provider failover,
- proxy configuration in provider profiles,
- chapter action execution or new AI workflows,
- persistent action runs or prompt/run logging,
- editing project-local agent/style YAML through the UI,
- changing the canonical patch acceptance transaction.

## Requirements

| ID | Requirement |
| --- | --- |
| M5-R01 | Strictly load and validate one application-level provider profile document outside story projects. |
| M5-R02 | Atomically save non-secret provider profiles with optimistic revision protection. |
| M5-R03 | Resolve bearer credentials only through an injected credential broker backed by process environment variables in production. |
| M5-R04 | Never accept, persist, return, or log provider credential values. |
| M5-R05 | Map provider-neutral generation requests to the exact OpenAI-compatible chat contract. |
| M5-R06 | Map provider-neutral generation requests to the exact Ollama chat contract. |
| M5-R07 | Enforce endpoint URL, redirect, timeout, request-size, response-size, and safe-error rules at the outbound boundary. |
| M5-R08 | Model agent/style/profile compatibility as a pure, inspectable decision using declared capabilities and credential readiness. |
| M5-R09 | Return only compatible, ready styles from action availability and revalidate them when a run starts. |
| M5-R10 | Route mock, OpenAI-compatible, and Ollama generation through one provider-neutral application boundary. |
| M5-R11 | Preserve minimal context and the existing transient preview, reject, and explicit accept behavior for real-provider runs. |
| M5-R12 | Expose the exact provider-profile API and extend existing registry/action responses as documented below. |
| M5-R13 | Provide an accessible provider settings UI with loading, empty, dirty, saving, saved, conflict, readiness, and error states. |
| M5-R14 | Preserve Milestone 0-4 behavior and keep full check, race, and diff-validation suites green. |
| M5-R15 | Maintain scenario-to-test evidence and complete all documentation and status updates before declaring the milestone complete. |

## BDD stories

### Story 5.1 - Configure provider profiles safely

As an author, I want to configure model endpoints without putting credentials in
my story folder so that projects remain portable and safe to share.

#### Scenario 5.1.1 - Start with no application configuration

Requirements: M5-R01, M5-R12, M5-R13.

```gherkin
Given the application provider configuration file does not exist
When I open provider settings
Then the API returns an empty profile list with a null revision
And the UI shows an empty state with an Add profile action
And no story project needs to be active
```

#### Scenario 5.1.2 - Save non-secret profiles

Requirements: M5-R01, M5-R02, M5-R04, M5-R12, M5-R13.

```gherkin
Given provider settings are loaded at their current revision
When I add or edit valid OpenAI-compatible and Ollama profiles and choose Save
Then the complete providers.yaml document is atomically replaced outside every story project
And its deterministic revision and normalized profiles are returned
And no Git repository, story file, SQLite index, or action run is changed
And the UI establishes the response as its clean Saved baseline
```

#### Scenario 5.1.3 - Reject invalid, stale, or malformed configuration

Requirements: M5-R01, M5-R02, M5-R04, M5-R12.

```gherkin
Given provider settings are loaded at revision A
When I submit invalid profile data or an unknown JSON field
Then the request returns 400 Bad Request without changing the configuration file
When canonical configuration is malformed
Then loading returns 500 Internal Server Error and does not repair or skip it
When canonical configuration changed to revision B and I save with revision A
Then the request returns 409 Conflict and revision B remains unchanged
```

#### Scenario 5.1.4 - Report credential readiness without exposing secrets

Requirements: M5-R03, M5-R04, M5-R12.

```gherkin
Given a bearer profile names an environment credential
When that environment variable is present
Then the profile status is ready
When it is absent or empty
Then the profile status is missing_credential
And neither response contains the credential value
And no log or error contains the credential value
```

### Story 5.2 - Filter actions by real model capabilities

As an author, I want styles offered only when their provider can execute the
agent so that incompatible actions fail before consuming time or money.

#### Scenario 5.2.1 - Match a ready provider

Requirements: M5-R08, M5-R09, M5-R12.

```gherkin
Given Line Polish is applicable to the canonical selection
And a project style references a ready chat-capable provider profile
And the profile context limit meets the agent minimum
When I request available actions
Then Line Polish includes that style ID
And the compatibility decision records that all requirements passed
```

#### Scenario 5.2.2 - Exclude an incompatible or unavailable style

Requirements: M5-R08, M5-R09.

```gherkin
Given a style references a missing profile, a missing bearer credential,
  a non-chat profile, or a profile below the minimum context limit
When compatibility is computed
Then the style is excluded with one stable inspectable reason
And it is absent from public action availability
And an action with no compatible styles is absent
```

#### Scenario 5.2.3 - Revalidate a selected style at run time

Requirements: M5-R03, M5-R08, M5-R09.

```gherkin
Given a style was previously available
When its profile or credential becomes unavailable before the run starts
Then the run fails before an outbound model request
And no transient run or project mutation is created
```

### Story 5.3 - Run an OpenAI-compatible provider

As an author, I want to use a local or API-key OpenAI-compatible endpoint so
that I can choose hosted or self-hosted models without changing the workflow.

#### Scenario 5.3.1 - Run a no-auth local endpoint

Requirements: M5-R05, M5-R07, M5-R10, M5-R11.

```gherkin
Given a ready no-auth OpenAI-compatible profile and compatible style
When I run Line Polish on a valid canonical selection
Then the adapter sends one non-streaming chat-completions request
And sends no Authorization header
And maps the first assistant text into the existing review patch
And no canonical file, index, staging state, or Git history changes
```

#### Scenario 5.3.2 - Run a bearer API-key endpoint

Requirements: M5-R03, M5-R04, M5-R05, M5-R07, M5-R10.

```gherkin
Given a ready bearer OpenAI-compatible profile
And its environment credential contains an API key
When I run Line Polish
Then the adapter sends that key only as an Authorization Bearer header
And the key appears in no request model, API response, error, file, database, or log
And the provider response becomes a transient review patch
```

#### Scenario 5.3.3 - Handle an unsafe or failed provider response

Requirements: M5-R04, M5-R07, M5-R11.

```gherkin
Given an OpenAI-compatible endpoint redirects, times out, returns an oversized body,
  returns malformed JSON, returns a non-success status, or returns empty assistant text
When I run the action
Then the adapter fails with the documented typed error
And no credential or provider response body is exposed
And no run or project mutation is created
```

### Story 5.4 - Run an Ollama provider

As an author, I want to use a local Ollama endpoint so that local models can
participate in the same controlled patch workflow.

#### Scenario 5.4.1 - Run Ollama chat

Requirements: M5-R06, M5-R07, M5-R10, M5-R11.

```gherkin
Given a ready Ollama profile and compatible style
When I run Line Polish on a valid canonical selection
Then the adapter sends one non-streaming Ollama chat request without credentials
And maps response.message.content into the existing review patch
And the context summary remains selected_text plus style_sheet with RAG mode none
And no canonical state changes until explicit acceptance
```

#### Scenario 5.4.2 - Handle Ollama failures safely

Requirements: M5-R06, M5-R07.

```gherkin
Given Ollama is unreachable or returns a non-success, oversized, malformed, or empty response
When I run the action
Then the action returns a safe provider error
And no transient run or project mutation is created
```

### Story 5.5 - Preserve author control in the editor

As an author, I want real providers to use the existing review workflow so that
switching providers never weakens canon protection.

#### Scenario 5.5.1 - Preview and reject real-provider output

Requirements: M5-R09, M5-R10, M5-R11, M5-R13.

```gherkin
Given a real-provider style is compatible with a clean canonical selection
When I run it and receive a replacement
Then the editor shows provider and model identity with the existing diff and context summary
When I reject the patch
Then the editor and all canonical state remain unchanged
```

#### Scenario 5.5.2 - Accept real-provider output explicitly

Requirements: M5-R10, M5-R11, M5-R14.

```gherkin
Given a real-provider patch is pending
When I explicitly accept it
Then the existing Milestone 4 revision-safe patch transaction changes only the selection
And exactly one Git checkpoint is created
And no second scene save is issued
```

#### Scenario 5.5.3 - Retain mock compatibility

Requirements: M5-R10, M5-R14.

```gherkin
Given an existing Milestone 4 project contains the version-1 Precise Editor mock style
When I run Line Polish
Then the deterministic mock path remains available and behaves as before
```

## Application configuration location

Provider profiles are global application configuration. Resolve the directory
once during production composition:

1. if `STORYWORK_CONFIG_DIR` is non-empty, use that absolute directory;
2. otherwise use `os.UserConfigDir()` joined with `storywork`.

Reject a relative `STORYWORK_CONFIG_DIR`. The one provider document is
`<config-dir>/providers.yaml`. Create the config directory with mode `0700` and
the file with mode `0600` on platforms that support Unix permissions. Although
the file contains no credentials, conservative permissions reduce endpoint and
environment-name disclosure.

Do not derive this path from the active project and do not place it under
`.storywork/`. Inject the resolved path or a profile-store boundary in tests;
tests must never read or write the developer's real user configuration.

A missing file is valid empty state. An existing empty, malformed, unsupported,
symlinked, or non-regular file is an internal configuration error. Do not
silently migrate, repair, partially load, or skip invalid profiles.

## Canonical provider profile contract

The complete strict YAML schema is:

```yaml
version: 1
profiles:
  - id: local_openai
    name: Local OpenAI-compatible
    type: openai_compatible
    base_url: http://127.0.0.1:1234/v1
    auth:
      type: none
      credential_env: ""
    capabilities:
      chat: true
      streaming: false
      structured_output: false
      max_context_tokens: 8192
  - id: local_ollama
    name: Local Ollama
    type: ollama
    base_url: http://127.0.0.1:11434
    auth:
      type: none
      credential_env: ""
    capabilities:
      chat: true
      streaming: false
      structured_output: false
      max_context_tokens: 8192
  - id: hosted_api
    name: Hosted API
    type: openai_compatible
    base_url: https://api.example.test/v1
    auth:
      type: bearer_env
      credential_env: STORYWORK_HOSTED_API_KEY
    capabilities:
      chat: true
      streaming: false
      structured_output: false
      max_context_tokens: 32768
```

Rules:

- Reject unknown and duplicate YAML fields at every object level and reject
  multiple YAML documents.
- `version` is exactly `1`.
- `profiles` is required and serializes as `[]` when empty.
- Profile IDs match `^[a-z][a-z0-9_]{0,63}$` and are unique.
- Names are trimmed, non-empty valid UTF-8, and at most 100 runes.
- Type is `openai_compatible` or `ollama`.
- `base_url` is an absolute `http` or `https` URL with a host and no user info,
  query, fragment, or trailing slash. Preserve a non-root path such as `/v1`.
- `auth.type` is `none` or `bearer_env`.
- `none` requires an empty `credential_env`.
- `bearer_env` is allowed only for `openai_compatible` and requires an HTTPS
  URL, except that HTTP is allowed for a loopback host.
- `credential_env` for `bearer_env` matches
  `^STORYWORK_[A-Z][A-Z0-9_]{0,127}$` and stores only the environment variable
  name, never its value.
- Ollama requires `auth.type: none` in Milestone 5.
- `capabilities.chat` must be true for a profile to execute current actions.
  False remains loadable so compatibility can report it.
- Streaming and structured output declarations are Boolean. Milestone 5
  adapters implement neither and current starter agents do not require them.
- `max_context_tokens` is between 1 and 10,000,000 inclusive.
- Sort canonical profiles by name then ID. Use two-space indentation and exactly
  one terminal newline.
- Revision is `sha256:` plus lowercase SHA-256 of exact canonical bytes. Missing
  file has null revision.
- A byte-identical save is a typed no-change error with no write.

Saving replaces the complete profile list. Use `expected_revision: null` only
when no file exists and the exact current revision otherwise. Serialize saves
within the provider store, re-read current bytes under the lock, compare the
revision, validate and normalize the full next document, then atomically write.
This application configuration mutation does not use story Git, story SQLite,
the active-project session, or the story mutation lock.

## Credential broker contract

Define the credential interface at its consumer boundary. It accepts a validated
environment reference, checks the supplied process environment lookup, trims no
credential bytes, and returns only availability plus an opaque credential value
to the outbound adapter. Production uses `os.LookupEnv`; tests inject a map or
fake broker.

An unset or empty value is missing. Never include the value in an error, format
it with `%v`, attach it to a response DTO, retain it in an action run, or log an
outbound request containing it. Do not cache credentials in Milestone 5; resolve
at availability/run time so process-environment changes are observed.

Public readiness values are:

- `ready` for no-auth profiles or bearer profiles with a non-empty credential,
- `missing_credential` for a bearer profile without one.

Readiness is advisory on list and authoritative again immediately before an
outbound request.

## Agent and style schema evolution

Continue loading Milestone 4 version-1 agents and styles exactly as before.
Add version 2 without silently reinterpreting version 1.

Version-2 agents add this required object:

```yaml
model_requirements:
  min_context_tokens: 2048
  supports_streaming: false
  supports_structured_output: false
```

All other agent fields retain the Milestone 4 schema. Version-1 agents receive
the effective requirements `min_context_tokens: 1`, streaming false, structured
output false, and chat true. Version-2 bounds are 1 through 10,000,000 tokens.

Version-2 styles retain the Milestone 4 fields:

```yaml
version: 2
id: local_precise_editor
name: Local Precise Editor
provider_profile_id: local_openai
model: local-model-name
parameters:
  temperature: 0.2
system_prompt: >
  You are a careful prose editor. Preserve facts, continuity, POV, and intent.
```

For version 2, `provider_profile_id` is any valid registry ID and `model` is a
trimmed, non-empty valid UTF-8 string of at most 200 runes with no control
characters. Temperature remains required and in 0 through 2. The system prompt
retains the Milestone 4 rules. Version-1 styles remain restricted to
`mock_default` and `mock`.

Style loading validates syntax only. A missing provider profile is not malformed
story canon; it is an incompatible style reported by compatibility decisions.
Do not make opening a project depend on global provider configuration.

Update new-project starter agent definitions to version 2 with explicit model
requirements. Retain the version-1 Precise Editor mock style so new and existing
projects always have an offline path. Do not add a real-provider style whose
profile/model cannot be known; authors add version-2 style YAML to their project
after configuring a matching profile.

## Pure compatibility rules

Compatibility input is one validated agent, style, optional provider profile,
and current credential readiness. Return `Compatible` plus a stable reason code,
not only a Boolean. Evaluate in this order:

1. `mock` - version-1 `mock_default`/`mock` is compatible with the built-in mock
   provider and does not require an application profile.
2. `profile_not_found` - a real style's provider profile ID is absent.
3. `missing_credential` - the profile requires an unavailable credential.
4. `chat_unsupported` - chat capability is false.
5. `context_limit_too_small` - profile maximum is below the agent minimum.
6. `streaming_unsupported` - the agent requires streaming and profile does not.
7. `structured_output_unsupported` - the agent requires structured output and
   profile does not.
8. `compatible` - every requirement passes.

Milestone 5 adapters do not execute agents that require streaming or structured
output even if a profile declares support; return a typed unsupported error at
run time. This prevents declared capability from being confused with adapter
implementation.

Availability keeps the Milestone 4 action applicability decision, filters each
style through compatibility, sorts compatible styles by name then ID, and omits
an action with no compatible styles. Public availability returns style IDs only.
Internal tests and future diagnostics inspect reason codes.

## Provider-neutral generation request

Keep provider-specific request/response structs inside their adapters. The
application request contains only:

- agent identity and instructions,
- style identity, system prompt, model, and temperature,
- selected canonical text in its typed context packet,
- context summary,
- provider profile ID as a routing reference.

The dispatcher resolves the profile and credential, rechecks compatibility,
and delegates to the mock, OpenAI-compatible, or Ollama adapter. The action
service depends on one generator interface and one compatibility/catalog
interface; it must not switch on provider type.

Use this exact provider-neutral prompt mapping for real providers:

- system message: the trimmed style system prompt,
- user message:

```text
Task: <agent description>

Rewrite only the selected text. Return replacement text only. Do not add commentary or Markdown fences.

Selected text:
<exact selected canonical text>
```

Do not include Codex, outline, file paths, profile configuration, credentials,
or run IDs. Do not trim the selected text before sending it. Provider output is
used as returned except line endings normalize to LF; reject invalid UTF-8, NUL,
empty/Unicode-whitespace-only output, output over 5 MiB, and a byte-identical
replacement before inserting a transient run.

## OpenAI-compatible adapter contract

Append `/chat/completions` to the normalized base URL path. Send:

```json
{
  "model": "local-model-name",
  "messages": [
    {"role": "system", "content": "Style system prompt"},
    {"role": "user", "content": "Task and selected text"}
  ],
  "temperature": 0.2,
  "stream": false
}
```

Use `Content-Type: application/json`. For `bearer_env`, set exactly one
`Authorization: Bearer <credential>` header. For `none`, send no Authorization
header. Decode only the first `choices[0].message.content` string from a 2xx
response; ignore provider-specific metadata and never leak its shape upward.

## Ollama adapter contract

Append `/api/chat` to the normalized base URL. Send:

```json
{
  "model": "local-model-name",
  "messages": [
    {"role": "system", "content": "Style system prompt"},
    {"role": "user", "content": "Task and selected text"}
  ],
  "stream": false,
  "options": {"temperature": 0.2}
}
```

Send no credentials. Decode only `message.content` from a 2xx response. Ignore
Ollama timing and token metadata.

## Outbound HTTP safety and error behavior

- Inject `*http.Client` or a narrow `Doer` in tests.
- Production client timeout is 60 seconds.
- Do not follow redirects. A 3xx response is a provider rejection; this also
  prevents forwarding bearer credentials to another host.
- Limit serialized outbound JSON to 6 MiB before sending.
- Limit response bodies to 2 MiB and detect overflow before JSON decoding.
- Require `application/json` when a successful response supplies Content-Type;
  tolerate a missing Content-Type for local compatibility.
- Strictly decode the needed response envelope, reject trailing JSON, and ignore
  additional provider response fields for compatibility.
- Close response bodies on every path.
- Respect request context cancellation.
- Never include response bodies, Authorization values, or selected text in
  returned errors or logs.

Typed provider failures map as follows:

- invalid or incompatible profile, style, model, capability, credential
  reference, or run input: `400 Bad Request`;
- upstream non-2xx, redirect, malformed success response, invalid/empty/oversized
  provider output, or unsupported success content type: `502 Bad Gateway`;
- connection failure, timeout, cancellation before output, or upstream `429` or
  `503`: `503 Service Unavailable`.

A byte-identical replacement retains the Milestone 4 no-change behavior and is
`400 Bad Request`; it is valid provider text but not a reviewable mutation.

All use the existing `{"error":"useful safe message"}` shape. A failure before
valid replacement output creates no run and changes no project state.

## Exact HTTP API

Provider settings routes do not require an active project.

```http
GET /api/provider-profiles
PUT /api/provider-profiles
```

GET response when configured:

```json
{
  "profiles": [
    {
      "id": "hosted_api",
      "name": "Hosted API",
      "type": "openai_compatible",
      "base_url": "https://api.example.test/v1",
      "auth": {"type": "bearer_env", "credential_env": "STORYWORK_HOSTED_API_KEY"},
      "capabilities": {
        "chat": true,
        "streaming": false,
        "structured_output": false,
        "max_context_tokens": 32768
      },
      "readiness": "ready"
    }
  ],
  "revision": "sha256:..."
}
```

Missing configuration returns `{"profiles":[],"revision":null}`.

PUT request replaces the complete list:

```json
{
  "profiles": [
    {
      "id": "local_ollama",
      "name": "Local Ollama",
      "type": "ollama",
      "base_url": "http://127.0.0.1:11434",
      "auth": {"type": "none", "credential_env": ""},
      "capabilities": {
        "chat": true,
        "streaming": false,
        "structured_output": false,
        "max_context_tokens": 8192
      }
    }
  ],
  "expected_revision": null
}
```

Success is `200 OK` with the GET response shape. Rules:

- reject unknown, missing, null, trailing, or wrongly typed fields at every
  object level except `expected_revision`, which is string or null;
- use a 1 MiB body limit;
- `400 Bad Request`: JSON, validation, no-change, or invalid revision shape;
- `409 Conflict`: stale expected revision;
- `413 Content Too Large`: request exceeds the body limit;
- `500 Internal Server Error`: path resolution, malformed canonical config,
  directory, permission, read, atomic-write, or sync failure;
- `405 Method Not Allowed`: known route with `Allow: GET, PUT`.

Do not add a credential field. Do not accept an Authorization header as a way to
configure a provider key.

Extend style list responses with `version`. For version-2 styles also return
`provider_readiness` as `ready`, `missing_profile`, or `missing_credential`.
Version-1 mock styles return `ready`. Never return a credential value.

Extend action run responses with:

```json
"provider":{"profile_id":"local_ollama","type":"ollama","model":"local-model-name"}
```

Store this non-secret provider identity in the transient run so the preview is
stable if configuration changes after generation. Accept/reject does not reload
the provider profile. Existing response fields and patch decision routes remain
unchanged.

## Provider settings UI

Add a top-level Provider settings workbench reachable before or after opening a
story project. Do not put provider form state in `App.tsx` beyond navigation.

Minimum UI:

- list profiles and public readiness,
- add, edit, remove, and reorder-independent profile rows,
- labeled controls for every non-secret field,
- type-dependent auth choices,
- explicit guidance that the key must be set in the named backend environment
  variable and cannot be entered in the browser,
- Save disabled while clean, invalid, or saving,
- loading, empty, dirty, saving, saved, conflict, and actionable error states,
- reload-on-conflict with discard confirmation for a dirty local draft,
- navigation confirmation and `beforeunload` while dirty,
- no connectivity test button in Milestone 5.

The scene editor continues to use the existing action flow. It shows provider
profile ID, provider type, and model in a pending preview, but never an endpoint
URL or credential reference. Dirty draft and stale-response rules remain
unchanged.

## Suggested package boundaries

Use cohesive boundaries rather than extending `internal/agent` into networking:

```text
internal/provider/    profile domain, strict app-config store, credential broker,
                      compatibility, dispatcher, OpenAI and Ollama adapters
internal/agent/       agent/style schemas and provider-neutral generation types
internal/action/      availability/run orchestration through injected interfaces
internal/api/         provider-profile HTTP transport and safe status mapping
internal/app/         config-path resolution and production dependency composition
web/src/providers/    provider settings workbench and focused tests
web/src/editor/       non-secret provider identity in existing preview
web/src/api.ts        typed provider-profile transport
```

Keep `cmd/storywork/main.go` limited to process startup. Do not create generic
`config`, `utils`, `common`, `manager`, or `client` packages. Split provider
files by responsibility (`model.go`, `store.go`, `credentials.go`,
`compatibility.go`, `openai.go`, `ollama.go`, `dispatcher.go`) when implemented.

## Required test architecture

Every Milestone 5 test file must state one BDD scenario, requirement IDs, and a
plain-English file purpose at the top. Every test function/case must have an
adjacent English `Test:` and `Requirements:` comment. The one real-adapter
acceptance file may be cross-scenario if every asserted step has adjacent
traceability.

Required layers:

- pure profile validation, normalization, URL, revision, and compatibility
  decision tests,
- strict YAML store tests with exact bytes, missing state, symlink refusal,
  optimistic conflict, atomic replacement, permissions, and concurrent saves,
- credential broker tests for unset, empty, present, and non-disclosure paths,
- exact outbound HTTP adapter tests using `httptest.Server`, including headers,
  JSON, paths, redirect refusal, cancellation, limits, safe errors, and cleanup,
- dispatcher tests proving mock/OpenAI/Ollama routing and run-time revalidation,
- action service tests for style filtering, readiness changes, no-op/invalid
  output, provider identity, and zero mutation before acceptance,
- API tests for exact JSON, required fields, limits, methods, statuses, and
  absence of credential values,
- frontend transport tests through intercepted `fetch`,
- provider settings component tests for all states and navigation protection,
- editor boundary tests proving provider identity display and unchanged
  reject/accept semantics,
- real filesystem/Git/SQLite plus local `httptest.Server` acceptance.

Do not make automated tests contact the public internet, a developer's Ollama
process, or a real paid API. Fakes alone are insufficient for outbound request
mapping, canonical config bytes, credential non-disclosure, and the complete
action/acceptance path.

## Ordered TDD implementation sequence

Use `.plans/milestone_5_implementation.md` as the working checklist. At a high
level, complete these slices in order:

1. baseline and requirement traceability scaffolding,
2. provider profile domain and strict app-config storage,
3. credential broker and non-disclosure guarantees,
4. version-2 agent/style schema and pure compatibility,
5. OpenAI-compatible adapter,
6. Ollama adapter,
7. dispatcher and action-service integration,
8. exact HTTP contract,
9. provider settings and editor UI integration,
10. real-adapter acceptance, evidence audit, docs, and final status.

Do not start the next slice until the current tests pass. When a test exposes a
contract flaw, update this durable document before implementing a different
behavior.

## Required automated acceptance

Create a Go integration test using a temporary application config directory,
temporary story project, real filesystem, real Git adapter, real SQLite index,
and local `httptest.Server` endpoints. It must:

1. prove missing provider config loads as empty without touching a story project;
2. save local OpenAI, Ollama, and bearer OpenAI-compatible profiles and verify
   exact canonical bytes, revision, location, permissions, and no secrets;
3. configure a fake environment credential and prove public responses never
   contain its value;
4. add compatible version-2 project styles and prove availability filters by
   capability and readiness;
5. run a local OpenAI-compatible action and prove exact outbound request plus
   zero canonical/index/Git mutation before decision;
6. reject that run and prove zero mutation;
7. run an Ollama action and prove exact outbound request and context summary;
8. run the bearer path and prove the key exists only in the outbound header;
9. accept one real-provider patch and prove exact selected-byte replacement,
   one checkpoint, clean worktree, rebuilt index, and new revision;
10. prove missing credential, redirect, timeout/cancellation, malformed response,
    stale config revision, stale scene revision, and dirty worktree create no
    unintended mutation;
11. reload provider configuration and scene state through fresh service
    instances and verify durable non-secret state and accepted canonical text;
12. rerun the existing mock flow to prove backward compatibility.

Frontend acceptance tests must exercise provider settings and the editor through
the fetch boundary, not by mocking their own API module.

## Documentation and status completion

Documentation and status updates are implementation work. Before marking
Milestone 5 complete:

1. update `docs/02_architecture.md` with the app-config store, credential broker,
   provider dispatcher, and adapter boundaries;
2. update `docs/03_storage_model.md` to distinguish application provider config
   from story-project canonical files and reiterate that credentials are absent;
3. update `docs/04_agent_style_system.md` with version-2 schemas, capability
   compatibility, real-provider behavior, and retained version-1 mock support;
4. update `docs/06_api_contract.md` with exact implemented provider routes,
   response extensions, and provider error statuses;
5. update `docs/07_frontend_editor.md` with provider settings and real-provider
   preview identity;
6. update `docs/08_testing_acceptance.md` with named Milestone 5 evidence;
7. update `DOCUMENTATION.md` to Version Milestone 5 and list implemented routes;
8. update `README.md` package map and implementation status to mark Milestone 5
   complete and Milestone 6 next;
9. update `docs/05_milestones.md` with completion date and next phase;
10. replace the initial contents of `.plans/milestone_5_test_evidence.md` with
    actual scenario-to-test mappings and honest residual limitations;
11. update `.plans/milestone_5_status.md` after every completed implementation
    slice, then record exact final commands, results, remaining risks, and the
    next incomplete milestone;
12. verify comments and docs describe implemented behavior rather than planned
    behavior, and remove stale Milestone 4-only claims where support expanded.

Do not mark status complete while any requirement is represented only by an
aspirational filename or an unrun manual step.

## Acceptance commands

Use the preferred Go toolchain and a writable cache:

```bash
PATH=/home/linuxbrew/.linuxbrew/bin:$PATH GOCACHE=/tmp/storywriter-go-cache make check
PATH=/home/linuxbrew/.linuxbrew/bin:$PATH GOCACHE=/tmp/storywriter-go-cache go test -race ./...
git diff --check
git status --short
```

Also verify that no credential value used by tests appears in tracked or
untracked repository files. Use fixed obvious test-only sentinel values and
search for those exact values; do not print the developer's environment.

Generated `web/dist`, databases, caches, temporary config directories, test
projects, credentials, and `node_modules` must remain untracked. Automated tests
must leave no server or process running.

## Definition of done

Milestone 5 is complete only when:

- every M5 requirement maps to implemented BDD scenarios and named passing tests,
- provider configuration is strict, atomic, revision-safe, outside story
  projects, and contains no credential value,
- the production credential broker resolves environment values without exposing
  or persisting them,
- capability/readiness decisions are pure and action availability fails closed,
- local OpenAI-compatible, bearer OpenAI-compatible, Ollama, and existing mock
  paths all use one provider-neutral generation boundary,
- outbound adapters enforce the documented mapping and safety limits,
- provider failure creates no transient run or canonical/index/Git mutation,
- real-provider output remains a transient patch until explicit acceptance,
- the provider settings and editor flows are proven through HTTP boundaries,
- full Milestone 0-4 regression and race suites pass,
- documentation, test evidence, and status artifacts contain actual results,
- no Milestone 6 behavior, credential, generated artifact, or live process is
  left behind.
