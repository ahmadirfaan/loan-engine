-- ============================================================
-- Mini P2P Lending Engine - Database Schema
-- ============================================================

-- Custom ENUM types
CREATE TYPE user_role AS ENUM ('BORROWER', 'INVESTOR', 'STAFF');
CREATE TYPE loan_status AS ENUM ('PROPOSED', 'APPROVED', 'INVESTED', 'DISBURSED');
CREATE TYPE document_type AS ENUM ('VISIT_PROOF', 'AGREEMENT');
CREATE TYPE outbox_status AS ENUM ('PENDING', 'SENT', 'FAILED');

-- ============================================================
-- USERS
-- ============================================================
CREATE TABLE "user" (
    id            BIGSERIAL PRIMARY KEY,
    name          VARCHAR(255) NOT NULL,
    role          user_role NOT NULL,
    email         VARCHAR(255) NOT NULL UNIQUE,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Seed some dummy users for testing
INSERT INTO "user" (name, role, email) VALUES
    ('Alice Borrower', 'BORROWER', 'alice@example.com'),
    ('Bob Investor', 'INVESTOR', 'bob@example.com'),
    ('Charlie Investor', 'INVESTOR', 'charlie@example.com'),
    ('Staff One', 'STAFF', 'staff1@example.com'),
    ('Staff Two', 'STAFF', 'staff2@example.com');

-- ============================================================
-- PRODUCT
-- ============================================================
CREATE TABLE product (
    id                   BIGSERIAL PRIMARY KEY,
    product_name         VARCHAR(255) NOT NULL,
    tenor_length         INT NOT NULL,            -- in months
    payment_frequency    VARCHAR(50) NOT NULL,    -- e.g. MONTHLY, WEEKLY
    interest_rate        NUMERIC(5,2) NOT NULL,   -- percentage
    roi_rate             NUMERIC(5,2) NOT NULL,   -- percentage
    penalty_rate         NUMERIC(5,2) NOT NULL,   -- percentage
    min_principal_amount NUMERIC(15,2) NOT NULL,
    max_principal_amount NUMERIC(15,2) NOT NULL,
    is_active            BOOLEAN NOT NULL DEFAULT TRUE,
    created_at           TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMP NOT NULL DEFAULT NOW()
);

-- ============================================================
-- DOCUMENT
-- ============================================================
CREATE TABLE document (
    id          BIGSERIAL PRIMARY KEY,
    file_name   VARCHAR(255) NOT NULL,
    file_url    VARCHAR(512) NOT NULL,
    type        document_type NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

-- ============================================================
-- LOAN
-- ============================================================
CREATE TABLE loan (
    id                      BIGSERIAL PRIMARY KEY,
    borrower_id             BIGINT NOT NULL REFERENCES "user"(id),
    product_id              BIGINT NOT NULL REFERENCES product(id),
    principal_amount        NUMERIC(15,2) NOT NULL,
    remainder_amount        NUMERIC(15,2) NOT NULL,
    interest_rate           NUMERIC(5,2) NOT NULL,
    roi_rate                NUMERIC(5,2) NOT NULL,
    status                  loan_status NOT NULL DEFAULT 'PROPOSED',
    approved_by_staff_id    BIGINT REFERENCES "user"(id),
    visited_document_id     BIGINT REFERENCES document(id),
    approval_at             TIMESTAMP NULL,
    disbursed_by_staff_id   BIGINT REFERENCES "user"(id),
    agreement_document_id   BIGINT REFERENCES document(id),
    disbursed_at            TIMESTAMP NULL,
    version                 INT NOT NULL DEFAULT 1,
    created_at              TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMP NOT NULL DEFAULT NOW()
);

-- ============================================================
-- LOAN INVESTMENT
-- ============================================================
CREATE TABLE loan_investment (
    id                      BIGSERIAL PRIMARY KEY,
    loan_id                 BIGINT NOT NULL REFERENCES loan(id),
    investor_id             BIGINT NOT NULL REFERENCES "user"(id),
    amount                  NUMERIC(15,2) NOT NULL,
    roi_rate                NUMERIC(5,2) NOT NULL,
    document_id_agreement   BIGINT REFERENCES document(id),
    is_signed_agreement     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at              TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMP NOT NULL DEFAULT NOW()
);

-- ============================================================
-- ORDER EVENT (Transactional Outbox)
-- ============================================================
CREATE TABLE order_event (
    id          BIGSERIAL PRIMARY KEY,
    payload     JSONB NOT NULL,
    status      outbox_status NOT NULL DEFAULT 'PENDING',
    event_type  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Index for the outbox relay query
CREATE INDEX idx_order_event_status ON order_event(status) WHERE status = 'PENDING';
