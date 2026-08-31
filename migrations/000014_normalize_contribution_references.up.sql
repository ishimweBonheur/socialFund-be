UPDATE contributions
SET transaction_reference = upper(btrim(transaction_reference))
WHERE transaction_reference IS NOT NULL;

DROP INDEX IF EXISTS contributions_transaction_reference_unique;

CREATE UNIQUE INDEX contributions_transaction_reference_unique
ON contributions (upper(btrim(transaction_reference)))
WHERE transaction_reference IS NOT NULL AND btrim(transaction_reference) <> '';
