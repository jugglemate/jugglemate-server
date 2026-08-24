## 1. Default Translation Model Configuration

- [x] 1.1 Add `translation_model -> default_translation_model` to the default model registry and expose `translation_model` in `DefaultModels`.
- [x] 1.2 Update defaults get/set handling to read, allow, store, and clear the translation default.
- [x] 1.3 Validate that a non-null translation default references an active model while skipping model capability matching.
- [x] 1.4 Add focused tests for reading, setting, clearing, and rejecting invalid translation defaults.

## 2. Translation Service And Contract

- [x] 2.1 Add translation request/response DTOs and a narrow model-caller interface for service tests.
- [x] 2.2 Implement strict request validation for source text, language labels, glossary size, and glossary terms using Unicode character counts.
- [x] 2.3 Implement default translation model lookup with the required no-default error and no fallback behavior.
- [x] 2.4 Build the fixed system prompt and JSON user payload while preserving the original source content.
- [x] 2.5 Invoke `CallService.Call` with the resolved model, `temperature=0.2`, and `metadata.call_type=translation`, then map content, actual model ID, usage, cost, and response time.

## 3. HTTP API And Dependency Injection

- [x] 3.1 Inject `TranslateService` into the LLM handler and register `POST /llm/translate`.
- [x] 3.2 Bind requests with unknown-field rejection so caller-supplied `model_id` and misspelled fields fail validation.
- [x] 3.3 Return validation/service errors and successful translations through the existing response envelope.
- [x] 3.4 Construct and inject `TranslateService` in Agent bootstrap and update affected handler construction tests.
- [x] 3.5 Verify the native route remains protected by existing `X-Internal-Auth` middleware.

## 4. Translation Call Facts And Migration

- [x] 4.1 Allow `translation` in the `CallService.record` call-type whitelist.
- [x] 4.2 Add a forward migration that recreates `ck_llm_model_calls_call_type` with `translation` allowed.
- [x] 4.3 Add or update persistence coverage to verify translation facts retain their call type and metrics.

## 5. Tests

- [x] 5.1 Add service tests for missing/default model resolution, no fallback, prompt JSON, optional fields, temperature, metadata, and response mapping.
- [x] 5.2 Add validation tests for blank/oversized source and languages plus invalid glossary entries and counts.
- [x] 5.3 Add handler tests for route registration, valid requests, required fields, unknown fields, and `model_id` rejection.
- [x] 5.4 Run focused LLM service/API tests and resolve regressions.
- [x] 5.5 Run `go test ./...`.

## 6. Documentation And Verification

- [x] 6.1 Document JuggleRouter Provider/model/default configuration using the existing administration APIs and model page.
- [x] 6.2 Add a manual curl example for authenticated translation and expected diagnostics.
- [ ] 6.3 Manually verify `llm_model_calls.call_type=translation` and that credentials/source text are not added to ordinary logs.
- [x] 6.4 Run `openspec validate add-llm-translation-api --strict`.

## 7. Translation Reasoning Mode

- [x] 7.1 Extend non-streaming model requests with optional `reasoning_effort` and forward it to OpenAI-compatible Providers.
- [x] 7.2 Set translation calls to `reasoning_effort=none` while leaving Anthropic native thinking disabled.
- [x] 7.3 Add service and gateway tests and rerun focused/full validation.
