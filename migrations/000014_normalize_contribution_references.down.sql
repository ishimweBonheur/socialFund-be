DROP INDEX IF EXISTS contributions_transaction_reference_unique;

CREATE UNIQUE INDEX contributions_transaction_reference_unique
ON contributions (transaction_reference)
WHERE transaction_reference IS NOT NULL;
