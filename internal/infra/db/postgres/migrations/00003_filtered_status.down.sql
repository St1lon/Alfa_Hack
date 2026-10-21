UPDATE news SET status_id = 7 WHERE status_id = 8;
UPDATE stories SET status_id = 7 WHERE status_id = 8;
DELETE FROM pipeline_statuses WHERE id = 8;
