## Why

Internal services need a stable text translation API backed by the centrally managed LLM provider configuration. Callers should not choose models or handle JuggleRouter credentials; administrators should select one default translation model and the server should consistently record usage and cost.

## What Changes

- Add internal authenticated endpoint `POST /api/v1/llm/translate` for synchronous text translation.
- Accept source text, target language, optional source language, and an optional per-request glossary; do not accept a caller-selected model.
- Add `translation_model` to the existing default LLM model configuration, stored as `default_translation_model` in `system_config`.
- Resolve the configured default translation model, build a fixed translation prompt, and call the existing non-streaming `CallService` with `temperature=0.2`.
- Return translated content, the actual model ID, token usage, cost, and response time.
- Record translation calls in `llm_model_calls` with `call_type=translation` and extend the existing database constraint accordingly.
- Add focused service, API, configuration, and persistence tests plus JuggleRouter setup and curl verification guidance.

## Capabilities

### New Capabilities
- `llm-text-translation`: Defines the internal translation API, default model resolution, request validation, prompt behavior, response metrics, and translation call recording.

### Modified Capabilities

None.

## Impact

- LLM DTOs, registry/default configuration, call recording, and HTTP handlers under `agent/modules/llm`.
- Agent bootstrap dependency injection and native `/api/v1` route registration.
- A forward database migration for the `llm_model_calls.call_type` check constraint.
- Existing provider/model administration remains the source of JuggleRouter credentials and model metadata; no translation job tables, file upload flow, streaming endpoint, or new model capability are introduced.
