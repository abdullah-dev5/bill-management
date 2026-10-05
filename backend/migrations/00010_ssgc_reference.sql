-- +goose Up
-- SSGC prints a 10-digit "Customer Number" (confirmed against a Sep 2024 sample bill); the digit in
-- brackets after it is a check digit and is not part of the number.
UPDATE providers SET ref_number_label = 'Customer No.', ref_number_pattern = '^\d{10}$'
WHERE id = 'b0000000-0000-4000-8000-000000000022';

-- +goose Down
UPDATE providers SET ref_number_label = 'Consumer No.', ref_number_pattern = NULL
WHERE id = 'b0000000-0000-4000-8000-000000000022';
