-- One debounce burst must have exactly one durable dispatch job. Keep one
-- row for each burst when upgrading a database that was written by the
-- previous, non-idempotent CreateDispatchJob implementation, then enforce
-- that invariant for every future insert.
DELETE FROM automation_dispatch_jobs
WHERE id NOT IN (
    SELECT MIN(id)
    FROM automation_dispatch_jobs
    GROUP BY chat_id, burst_version
);

CREATE UNIQUE INDEX automation_dispatch_jobs_burst_idx
    ON automation_dispatch_jobs(chat_id, burst_version);
