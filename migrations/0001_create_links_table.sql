-- Create links table
CREATE TABLE links (
    code VARCHAR(10) PRIMARY KEY,
    long_url TEXT NOT NULL,
    alias VARCHAR(50) UNIQUE,
    password_hash VARCHAR(255),
    expires_at TIMESTAMPTZ,
    max_clicks INTEGER,
    click_count INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    owner_id VARCHAR(50)
);

-- Indexes
CREATE INDEX idx_links_code ON links(code);
CREATE INDEX idx_links_expires_at ON links(expires_at);
CREATE INDEX idx_links_alias ON links(alias);

-- Sequence for code generation
CREATE SEQUENCE link_code_seq START 1;