-- Add 'stdout' to the provider_type enum.
-- Must be in its own migration because ALTER TYPE ADD VALUE cannot be
-- used in the same transaction as statements referencing the new value.
ALTER TYPE provider_type ADD VALUE IF NOT EXISTS 'stdout';
