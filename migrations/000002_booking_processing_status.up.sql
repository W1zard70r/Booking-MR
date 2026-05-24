ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_status_check;
ALTER TABLE bookings ADD CONSTRAINT bookings_status_check CHECK (status IN ('processing', 'active', 'cancelled'));

DROP INDEX IF EXISTS idx_unique_active_booking;
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_active_booking ON bookings (slot_id) WHERE status IN ('processing', 'active');
