UPDATE service_skus
SET rules = JSON_REMOVE(COALESCE(rules, JSON_OBJECT()), '$.duration_options'),
    updated_at = NOW(3)
WHERE service_type = 'dogwalk'
  AND plan = 'single';
