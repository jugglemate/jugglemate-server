ALTER TABLE llm_model_calls
  DROP CONSTRAINT IF EXISTS ck_llm_model_calls_call_type;

ALTER TABLE llm_model_calls
  ADD CONSTRAINT ck_llm_model_calls_call_type
  CHECK (call_type IN ('reasoning', 'summary', 'embedding', 'translation'));
