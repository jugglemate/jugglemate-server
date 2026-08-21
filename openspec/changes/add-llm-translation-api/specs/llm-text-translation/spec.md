## ADDED Requirements

### Requirement: Internal text translation endpoint
The system SHALL expose `POST /api/v1/llm/translate` through the native Agent API and SHALL protect it with the existing `X-Internal-Auth` authentication middleware.

#### Scenario: Authenticated translation request
- **WHEN** an internal caller supplies valid internal authentication and a valid translation request
- **THEN** the system invokes the translation service and returns the standard success envelope

#### Scenario: Unauthenticated translation request
- **WHEN** a caller omits or supplies invalid internal authentication
- **THEN** the existing native authentication middleware rejects the request before translation executes

### Requirement: Translation request contract
The system SHALL accept `source` and `target_language`, SHALL accept optional `source_language` and `glossary`, and SHALL reject unknown fields including `model_id`.

#### Scenario: Valid minimal request
- **WHEN** a caller supplies non-blank `source` and `target_language`
- **THEN** the request is accepted with automatic source-language detection

#### Scenario: Valid glossary request
- **WHEN** a caller supplies a bounded string-to-string glossary
- **THEN** the glossary is included as translation data

#### Scenario: Caller attempts to select a model
- **WHEN** a request contains `model_id`
- **THEN** the system rejects it as an unknown field

### Requirement: Translation input limits
The system SHALL reject blank source or target language, source text over 20,000 Unicode characters, language labels over 64 Unicode characters, glossaries over 100 entries, blank glossary terms or translations, glossary terms over 128 Unicode characters, and glossary translations over 256 Unicode characters.

#### Scenario: Blank required field
- **WHEN** source or target language contains only whitespace
- **THEN** the system returns a validation error without invoking the model

#### Scenario: Oversized source
- **WHEN** source contains more than 20,000 Unicode characters
- **THEN** the system returns a validation error without invoking the model

#### Scenario: Invalid glossary
- **WHEN** the glossary violates an entry count, blank value, or term length limit
- **THEN** the system returns a validation error without invoking the model

### Requirement: Administrator-controlled default translation model
The system SHALL expose `translation_model` through the existing LLM defaults API, SHALL store it under `default_translation_model`, and SHALL require a referenced model to exist and be active without requiring a translation-specific capability.

#### Scenario: Set active default translation model
- **WHEN** an administrator sets `translation_model` to an existing active model ID
- **THEN** the system stores that ID as `default_translation_model` and returns it in the defaults response

#### Scenario: Set inactive or missing model
- **WHEN** an administrator sets `translation_model` to a missing or inactive model
- **THEN** the system rejects the configuration change

### Requirement: Strict default model resolution
The translation service SHALL use only `default_translation_model` and SHALL NOT accept caller model selection or fall back to any other model.

#### Scenario: Default model configured
- **WHEN** `default_translation_model` contains a model ID
- **THEN** the service passes that exact model ID to `CallService.Call`

#### Scenario: Default model absent
- **WHEN** `default_translation_model` is absent, null, or blank
- **THEN** the system returns `未配置默认翻译模型，请先在管理后台设置` without invoking a model

### Requirement: Controlled translation prompt
The translation service SHALL use a fixed professional translation system instruction, SHALL encode source language, target language, glossary, and source text as JSON data, SHALL preserve the original source content, and SHALL invoke the model with `temperature=0.2` and `metadata.call_type=translation`.

#### Scenario: Translation with all optional data
- **WHEN** source language and glossary are supplied
- **THEN** the model request contains those values as JSON data, the original source, the fixed system instruction, temperature 0.2, and translation metadata

#### Scenario: Automatic source-language detection
- **WHEN** source language is omitted
- **THEN** the model prompt omits the source-language field and instructs the model to translate the supplied source into the target language

### Requirement: Translation response diagnostics
The system SHALL return translated `content`, the actual configured `model_id`, `usage`, `cost`, and `response_time_ms` in the standard response envelope.

#### Scenario: Successful model call
- **WHEN** `CallService.Call` returns successfully
- **THEN** the translation response maps its content, usage, cost, and response time and adds the resolved default model ID

### Requirement: Translation call facts
The system SHALL permit and persist `translation` as an `llm_model_calls.call_type` for successful, failed, and timed-out translation model calls.

#### Scenario: Successful translation fact
- **WHEN** a translation model call succeeds
- **THEN** its model, Provider, token usage, cost, timing, status, and `call_type=translation` are persisted through the existing call recording flow

#### Scenario: Failed translation fact
- **WHEN** the upstream translation model call fails after model resolution
- **THEN** the existing call recording flow persists a failed or timed-out fact with `call_type=translation`
