-- +goose Up
-- System bill types (household_id NULL). IDs are fixed so code and fixtures can refer to them.
INSERT INTO categories (id, household_id, name, icon, default_kind, type_group, parent_id, sort_order) VALUES
    -- Utilities
    ('a0000000-0000-4000-8000-000000000001', NULL, 'Electricity',        'zap',            'metered',     'utilities', NULL, 10),
    ('a0000000-0000-4000-8000-000000000002', NULL, 'Gas',                'flame',          'metered',     'utilities', NULL, 20),
    ('a0000000-0000-4000-8000-000000000003', NULL, 'Water',              'droplet',        'metered',     'utilities', NULL, 30),
    ('a0000000-0000-4000-8000-000000000004', NULL, 'Internet',           'wifi',           'recurring',   'utilities', NULL, 40),
    ('a0000000-0000-4000-8000-000000000005', NULL, 'Mobile / Telephone', 'smartphone',     'recurring',   'utilities', NULL, 50),
    ('a0000000-0000-4000-8000-000000000006', NULL, 'Cable / TV',         'tv',             'recurring',   'utilities', NULL, 60),
    -- Household
    ('a0000000-0000-4000-8000-000000000011', NULL, 'Rent',               'house',          'recurring',   'household', NULL, 110),
    ('a0000000-0000-4000-8000-000000000012', NULL, 'Maintenance',        'wrench',         'expense_log', 'household', NULL, 120),
    ('a0000000-0000-4000-8000-000000000013', NULL, 'Repairs',            'hammer',         'expense_log', 'household', NULL, 130),
    ('a0000000-0000-4000-8000-000000000014', NULL, 'Cleaning',           'sparkles',       'expense_log', 'household', NULL, 140),
    ('a0000000-0000-4000-8000-000000000015', NULL, 'Security',           'shield',         'recurring',   'household', NULL, 150),
    ('a0000000-0000-4000-8000-000000000016', NULL, 'Appliances',         'refrigerator',   'expense_log', 'household', NULL, 160),
    -- Other
    ('a0000000-0000-4000-8000-000000000021', NULL, 'Insurance',          'umbrella',       'recurring',   'other',     NULL, 210),
    ('a0000000-0000-4000-8000-000000000022', NULL, 'Education',          'graduation-cap', 'recurring',   'other',     NULL, 220),
    ('a0000000-0000-4000-8000-000000000023', NULL, 'Subscriptions',      'repeat',         'recurring',   'other',     NULL, 230),
    ('a0000000-0000-4000-8000-000000000024', NULL, 'Government fees',    'landmark',       'expense_log', 'other',     NULL, 240),
    ('a0000000-0000-4000-8000-000000000025', NULL, 'General expenses',   'receipt',        'expense_log', 'other',     NULL, 250),
    ('a0000000-0000-4000-8000-000000000026', NULL, 'Other',              'ellipsis',       'expense_log', 'other',     NULL, 260),
    -- Sub-categories of Maintenance (used inside expense-log profiles)
    ('a0000000-0000-4000-8000-000000000031', NULL, 'Electrician',        'plug-zap',       'expense_log', 'household', 'a0000000-0000-4000-8000-000000000012', 310),
    ('a0000000-0000-4000-8000-000000000032', NULL, 'Plumber',            'droplets',       'expense_log', 'household', 'a0000000-0000-4000-8000-000000000012', 320),
    ('a0000000-0000-4000-8000-000000000033', NULL, 'Painting',           'paint-roller',   'expense_log', 'household', 'a0000000-0000-4000-8000-000000000012', 330),
    ('a0000000-0000-4000-8000-000000000034', NULL, 'Carpentry',          'hammer',         'expense_log', 'household', 'a0000000-0000-4000-8000-000000000012', 340);

-- Provider catalogue (Pakistan). Reference-number patterns apply to digits-only normalised numbers;
-- NULL means "not validated yet" until confirmed against real sample bills (M0 fixture set).
INSERT INTO providers (id, household_id, name, short_name, category_id, region, ref_number_label, ref_number_pattern, consumption_unit) VALUES
    -- Electricity (DISCOs share the PITC 14-digit reference number)
    ('b0000000-0000-4000-8000-000000000001', NULL, 'Lahore Electric Supply Company',     'LESCO', 'a0000000-0000-4000-8000-000000000001', 'Lahore',      'Reference No.', '^\d{14}$', 'kWh'),
    ('b0000000-0000-4000-8000-000000000002', NULL, 'Islamabad Electric Supply Company',  'IESCO', 'a0000000-0000-4000-8000-000000000001', 'Islamabad',   'Reference No.', '^\d{14}$', 'kWh'),
    ('b0000000-0000-4000-8000-000000000003', NULL, 'Gujranwala Electric Power Company',  'GEPCO', 'a0000000-0000-4000-8000-000000000001', 'Gujranwala',  'Reference No.', '^\d{14}$', 'kWh'),
    ('b0000000-0000-4000-8000-000000000004', NULL, 'Faisalabad Electric Supply Company', 'FESCO', 'a0000000-0000-4000-8000-000000000001', 'Faisalabad',  'Reference No.', '^\d{14}$', 'kWh'),
    ('b0000000-0000-4000-8000-000000000005', NULL, 'Multan Electric Power Company',      'MEPCO', 'a0000000-0000-4000-8000-000000000001', 'Multan',      'Reference No.', '^\d{14}$', 'kWh'),
    ('b0000000-0000-4000-8000-000000000006', NULL, 'Peshawar Electric Supply Company',   'PESCO', 'a0000000-0000-4000-8000-000000000001', 'Peshawar',    'Reference No.', '^\d{14}$', 'kWh'),
    ('b0000000-0000-4000-8000-000000000007', NULL, 'Hyderabad Electric Supply Company',  'HESCO', 'a0000000-0000-4000-8000-000000000001', 'Hyderabad',   'Reference No.', '^\d{14}$', 'kWh'),
    ('b0000000-0000-4000-8000-000000000008', NULL, 'Quetta Electric Supply Company',     'QESCO', 'a0000000-0000-4000-8000-000000000001', 'Quetta',      'Reference No.', '^\d{14}$', 'kWh'),
    ('b0000000-0000-4000-8000-000000000009', NULL, 'Sukkur Electric Power Company',      'SEPCO', 'a0000000-0000-4000-8000-000000000001', 'Sukkur',      'Reference No.', '^\d{14}$', 'kWh'),
    ('b0000000-0000-4000-8000-000000000010', NULL, 'Tribal Electric Supply Company',     'TESCO', 'a0000000-0000-4000-8000-000000000001', 'Tribal areas','Reference No.', '^\d{14}$', 'kWh'),
    ('b0000000-0000-4000-8000-000000000011', NULL, 'K-Electric',                         'K-Electric', 'a0000000-0000-4000-8000-000000000001', 'Karachi', 'Account No.', NULL, 'kWh'),
    -- Gas
    ('b0000000-0000-4000-8000-000000000021', NULL, 'Sui Northern Gas Pipelines',         'SNGPL', 'a0000000-0000-4000-8000-000000000002', 'Punjab / KP', 'Consumer No.', NULL, 'hm³'),
    ('b0000000-0000-4000-8000-000000000022', NULL, 'Sui Southern Gas Company',           'SSGC',  'a0000000-0000-4000-8000-000000000002', 'Sindh / Balochistan', 'Consumer No.', NULL, 'hm³'),
    -- Water
    ('b0000000-0000-4000-8000-000000000031', NULL, 'WASA Lahore',                        'WASA Lahore', 'a0000000-0000-4000-8000-000000000003', 'Lahore', 'Account No.', NULL, NULL),
    ('b0000000-0000-4000-8000-000000000032', NULL, 'WASA Rawalpindi',                    'WASA Rawalpindi', 'a0000000-0000-4000-8000-000000000003', 'Rawalpindi', 'Account No.', NULL, NULL),
    ('b0000000-0000-4000-8000-000000000033', NULL, 'Karachi Water & Sewerage Corporation','KW&SC', 'a0000000-0000-4000-8000-000000000003', 'Karachi', 'Consumer No.', NULL, NULL),
    -- Internet & phone
    ('b0000000-0000-4000-8000-000000000041', NULL, 'PTCL',                               'PTCL',       'a0000000-0000-4000-8000-000000000004', NULL, 'Account ID', NULL, NULL),
    ('b0000000-0000-4000-8000-000000000042', NULL, 'Nayatel',                            'Nayatel',    'a0000000-0000-4000-8000-000000000004', NULL, 'Customer ID', NULL, NULL),
    ('b0000000-0000-4000-8000-000000000043', NULL, 'StormFiber',                         'StormFiber', 'a0000000-0000-4000-8000-000000000004', NULL, 'Customer ID', NULL, NULL),
    ('b0000000-0000-4000-8000-000000000051', NULL, 'Jazz',                               'Jazz',       'a0000000-0000-4000-8000-000000000005', NULL, 'Mobile No.', NULL, NULL),
    ('b0000000-0000-4000-8000-000000000052', NULL, 'Zong',                               'Zong',       'a0000000-0000-4000-8000-000000000005', NULL, 'Mobile No.', NULL, NULL),
    ('b0000000-0000-4000-8000-000000000053', NULL, 'Ufone',                              'Ufone',      'a0000000-0000-4000-8000-000000000005', NULL, 'Mobile No.', NULL, NULL),
    ('b0000000-0000-4000-8000-000000000054', NULL, 'Telenor',                            'Telenor',    'a0000000-0000-4000-8000-000000000005', NULL, 'Mobile No.', NULL, NULL);

-- +goose Down
DELETE FROM providers WHERE household_id IS NULL;
DELETE FROM categories WHERE household_id IS NULL AND parent_id IS NOT NULL;
DELETE FROM categories WHERE household_id IS NULL;
