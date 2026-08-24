## Context

The Agent LLM module already manages encrypted Provider credentials, OpenAI-compatible endpoints, models, system defaults, non-streaming calls, token/cost calculation, and `llm_model_calls` facts. Internal native routes are protected by `X-Internal-Auth`. This change adds a small synchronous text translation facade over those facilities instead of introducing a second JuggleRouter client or a translation job domain.

The caller must not select a model. Administrators select an active model through the existing defaults API and the translation service reads `system_config.default_translation_model` for every request. JuggleRouter is configured as an `openai-compatible` Provider with `https://jugglerouter.com/v1` and an encrypted API key.

## Goals / Non-Goals

**Goals:**

- Expose `POST /api/v1/llm/translate` under existing internal authentication.
- Validate bounded source text, language labels, and a bounded optional glossary.
- Resolve only the explicitly configured default translation model and return a clear error when absent.
- Reuse `CallService.Call`, including Provider credential decryption, active-state checks, usage, cost, timing, and facts.
- Produce a deterministic, injection-resistant prompt with `temperature=0.2`, disable deep reasoning, and return only translated content plus diagnostics.
- Record successful and failed upstream calls as `call_type=translation`.

**Non-Goals:**

- File upload, document parsing, async tasks, chunking, streaming, OCR, or format preservation.
- Caller-selected models, fallback model selection, or automatic default selection when a Provider changes.
- A `translation` model capability or changes to the model administration form.
- Persistent glossary management or storage of translation source/result text.

## Decisions

1. **Add a dedicated `TranslateService` in the existing LLM module.**
   - The service owns default resolution and prompt construction while `CallService` remains the sole model execution path.
   - It depends on a narrow caller interface so prompt/model behavior can be unit tested without a live Provider.
   - Alternative considered: put translation directly in the HTTP handler. That would mix validation, configuration lookup, and model orchestration and make focused testing harder.

2. **Use `translation_model` as the defaults API field and `default_translation_model` as the storage key.**
   - This follows the existing reasoning/summary/embedding naming convention.
   - Setting this default requires an active model but intentionally skips capability matching because no new translation capability is introduced.
   - Runtime execution still rejects an inactive Provider through `CallService.resolve`.

3. **Fail closed when no default translation model is configured.**
   - Return `未配置默认翻译模型，请先在管理后台设置` without invoking any model.
   - Do not fall back to reasoning defaults or arbitrary active models; silent fallback can change cost, quality, and upstream data handling.

4. **Serialize translation input as JSON inside a fixed prompt.**
   - A fixed system instruction treats source text, language labels, and glossary entries as untrusted data and requests translated content only.
   - JSON serialization avoids delimiter ambiguity and nondeterministic manual map iteration.
   - The original source is preserved for translation; trimming is used only to detect blank input.

5. **Disable deep reasoning for translation calls.**
   - Translation sets `reasoning_effort=none` for OpenAI-compatible Providers such as JuggleRouter to reduce latency and avoid unnecessary reasoning tokens.
   - The Anthropic native protocol omits its optional thinking configuration, which is the equivalent non-thinking mode for that protocol.

6. **Reject unknown request fields.**
   - The handler uses a strict JSON decoder so `model_id` and misspelled fields fail explicitly rather than being silently ignored.
   - Input limits use Unicode rune counts and cap source text, language labels, glossary entries, and glossary term lengths.

7. **Extend the existing call-type database constraint.**
   - `CallService.record` recognizes `translation`, and a forward migration recreates `ck_llm_model_calls_call_type` with the new value.
   - No table or column is added.

8. **Return the resolved model ID from `TranslateService`.**
   - The generic `CallResponse` remains unchanged; the translation response combines it with the exact default model ID captured before the call.

## Risks / Trade-offs

- [Risk] A synchronous request may exceed gateway timeouts for large text. → Cap source length at 20,000 Unicode characters and rely on configured Provider timeout; document/file translation remains out of scope.
- [Risk] Free-form language labels can be ambiguous. → Bound their length and pass them as data; standardized language codes can be added later without breaking the basic API.
- [Risk] Prompt injection inside source text may influence the model. → Use a fixed system instruction that treats all request fields as data and asks for translated content only; no tools are supplied.
- [Risk] Translation facts fail to persist if code deploys before the constraint migration. → Apply the database migration before serving the new endpoint.
- [Risk] `CallService` records upstream error text. → This change does not add source or glossary content to metadata or logs; broader upstream error sanitization is outside this change.

## Migration Plan

1. Apply the migration that permits `call_type=translation`.
2. Deploy DTO, registry, service, handler, and bootstrap changes.
3. Configure JuggleRouter Provider and active model through the existing administration flow.
4. Set `translation_model` through `PUT /api/v1/admin/llm/defaults`.
5. Verify translation and the corresponding model-call fact with an internal curl request.

Rollback disables/removes the route and service code. The added check-constraint value and optional `system_config` row are backward compatible and may remain; the configuration can be cleared through the defaults API.

## Open Questions

- None for this scope. File translation, standardized language catalogs, and persistent glossaries require separate changes.
