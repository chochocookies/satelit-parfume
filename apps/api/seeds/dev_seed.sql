-- ══════════════════════════════════════════════════════════════════════
-- DEVELOPMENT SEED DATA — NOT Satelit Parfume's real staff or customers.
-- Do not run this against a production database.
-- ══════════════════════════════════════════════════════════════════════
--
-- Creates one login for each side of the auth system so Phase 2 is
-- actually testable end-to-end:
--   Staff:    admin@satelitparfume.dev     / ChangeMe123!  (SUPER_ADMIN)
--   Customer: customer@satelitparfume.dev  / ChangeMe123!
--
-- The password hash below is a real bcrypt hash of "ChangeMe123!" (cost
-- 10) — generated once and pasted in, not a placeholder string. Change or
-- delete these accounts before this ever points at production data.

INSERT INTO users (name, email, password_hash, status)
VALUES (
    'Dev Super Admin',
    'admin@satelitparfume.dev',
    '$2b$10$kQiWe/6CiFml/isRl5HQhePkdWAifmD4W6XYUZwAdAWEGUSECbt.i',
    'active'
)
ON CONFLICT (email) DO NOTHING;

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u, roles r
WHERE u.email = 'admin@satelitparfume.dev'
  AND r.name = 'SUPER_ADMIN'
ON CONFLICT DO NOTHING;

INSERT INTO customers (name, email, password_hash, status)
VALUES (
    'Dev Customer',
    'customer@satelitparfume.dev',
    '$2b$10$kQiWe/6CiFml/isRl5HQhePkdWAifmD4W6XYUZwAdAWEGUSECbt.i',
    'active'
)
ON CONFLICT (email) DO NOTHING;
