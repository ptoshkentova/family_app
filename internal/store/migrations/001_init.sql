-- One budget per Telegram chat: a private chat is a personal budget, a group is a family budget.
CREATE TABLE budgets (
    chat_id    BIGINT PRIMARY KEY,
    title      TEXT        NOT NULL,
    is_group   BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE members (
    chat_id    BIGINT      NOT NULL REFERENCES budgets (chat_id) ON DELETE CASCADE,
    user_id    BIGINT      NOT NULL,
    first_name TEXT        NOT NULL,
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (chat_id, user_id)
);
CREATE INDEX members_user_idx ON members (user_id);

CREATE TABLE transactions (
    id          BIGSERIAL PRIMARY KEY,
    chat_id     BIGINT      NOT NULL REFERENCES budgets (chat_id) ON DELETE CASCADE,
    user_id     BIGINT      NOT NULL,
    user_name   TEXT        NOT NULL,
    amount      BIGINT      NOT NULL CHECK (amount > 0),
    category    TEXT        NOT NULL,
    description TEXT        NOT NULL,
    is_income   BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX transactions_chat_time_idx ON transactions (chat_id, created_at);
