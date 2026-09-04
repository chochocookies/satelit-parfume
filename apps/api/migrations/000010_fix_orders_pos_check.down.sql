ALTER TABLE orders DROP CONSTRAINT orders_check;

ALTER TABLE orders ADD CONSTRAINT orders_check
    CHECK (customer_id IS NOT NULL OR guest_name IS NOT NULL);
