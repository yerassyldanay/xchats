-- Reverses 0017_account_auto_response. Drop order respects FKs: jobs and
-- windows have no dependents, account_auto_response_window FKs into
-- account_auto_response, and chat_draft_debounce is independent of all three.
SET search_path = xchats, public;

DROP TABLE IF EXISTS xchats.auto_response_jobs;
DROP TABLE IF EXISTS xchats.account_auto_response_window;
DROP TABLE IF EXISTS xchats.account_auto_response;
DROP TABLE IF EXISTS xchats.chat_draft_debounce;
