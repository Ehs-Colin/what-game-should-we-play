-- +goose Up
CREATE TABLE steam_user_groups (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- Group names are unique without regard to case (exact View Group lookup).
CREATE UNIQUE INDEX steam_user_groups_name_lower ON steam_user_groups (lower(name));

CREATE TABLE steam_user_group_members (
    group_id BIGINT NOT NULL,
    steam_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (group_id, steam_id),
    FOREIGN KEY (group_id) REFERENCES steam_user_groups(id) ON DELETE CASCADE,
    FOREIGN KEY (steam_id) REFERENCES steam_users(steam_id) ON DELETE CASCADE
);

-- Reverse lookup for "groups this user belongs to".
CREATE INDEX steam_user_group_members_steam_id ON steam_user_group_members (steam_id);

-- +goose Down
DROP TABLE steam_user_group_members;
DROP TABLE steam_user_groups;
