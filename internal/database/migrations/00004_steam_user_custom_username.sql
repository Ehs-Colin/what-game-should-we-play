-- +goose Up
-- App-assigned labels for users imported by Steam ID. The flag marks rows
-- whose name is a custom username rather than a real Steam vanity. Custom
-- names are unique without regard to case among custom-named rows only;
-- vanity rows keep their existing non-unique name behavior.
ALTER TABLE steam_users
ADD COLUMN custom_username BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX steam_users_custom_name_unique
ON steam_users (lower(name))
WHERE custom_username;

-- +goose Down
DROP INDEX IF EXISTS steam_users_custom_name_unique;
ALTER TABLE steam_users DROP COLUMN custom_username;
