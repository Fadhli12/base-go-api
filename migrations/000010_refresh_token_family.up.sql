-- MED-004: Add token family tracking for refresh token rotation detection
-- This prevents reuse of revoked tokens by tracking token families

-- Add family_id column to refresh_tokens
ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS family_id UUID NOT NULL DEFAULT gen_random_uuid();

-- Create index on family_id for efficient lookups
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family_id ON refresh_tokens(family_id);

-- Drop the old default constraint if it exists (from previous migration)
DO $$ 
BEGIN
    -- Update any existing tokens without a family_id to have a new family
    -- This ensures backward compatibility
    UPDATE refresh_tokens 
    SET family_id = gen_random_uuid() 
    WHERE family_id IS NULL OR family_id = '00000000-0000-0000-0000-000000000000';
    
    -- Make family_id NOT NULL (already done above with DEFAULT, but be explicit)
    -- Note: Already NOT NULL in the ALTER TABLE above
END $$;