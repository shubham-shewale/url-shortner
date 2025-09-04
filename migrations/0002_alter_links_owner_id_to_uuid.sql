-- Alter owner_id column to UUID NOT NULL
ALTER TABLE links ALTER COLUMN owner_id TYPE UUID USING owner_id::UUID;
ALTER TABLE links ALTER COLUMN owner_id SET NOT NULL;