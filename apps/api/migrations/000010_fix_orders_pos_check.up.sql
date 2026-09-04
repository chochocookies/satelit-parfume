-- Fix: Phase 10 (migration 000008) added cashier_shift_id to mark a POS
-- sale, and internal/orders' Service.Checkout already treats a set
-- cashier_shift_id as sufficient identification on its own — a walk-in
-- POS sale collects no name or phone by design, the same way Phase 7's
-- own notes anticipated. But this table's CHECK constraint, from Phase
-- 7, was never updated to agree: it only ever accepted customer_id or
-- guest_name, so every real POS sale (which sets neither) has been
-- rejected at the database level with a raw "violates check constraint
-- orders_check" since Phase 10 shipped, regardless of what the
-- application layer already allowed through.
ALTER TABLE orders DROP CONSTRAINT orders_check;

ALTER TABLE orders ADD CONSTRAINT orders_check
    CHECK (customer_id IS NOT NULL OR guest_name IS NOT NULL OR cashier_shift_id IS NOT NULL);
