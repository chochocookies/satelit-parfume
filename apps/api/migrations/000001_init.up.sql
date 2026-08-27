-- Phase 1: bootstrap only. Enables pgcrypto so later migrations can use
-- gen_random_uuid() for primary keys, per the "prefer UUID for externally
-- exposed IDs" rule. Domain tables (users, branches, products, ...) are
-- intentionally NOT created here — they arrive with their own phases so
-- each migration stays reviewable and each phase stays independently
-- runnable.
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
