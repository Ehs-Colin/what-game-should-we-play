-- +goose Up
-- Steam profile fields from ISteamUser/GetPlayerSummaries. Nullable because
-- existing rows keep NULL until they are submitted through InsertSteamUser
-- again.
ALTER TABLE steam_users
    ADD COLUMN personaname VARCHAR(255),
    ADD COLUMN profileurl VARCHAR(255),
    ADD COLUMN avatar VARCHAR(255),
    ADD COLUMN avatar_medium VARCHAR(255),
    ADD COLUMN avatar_full VARCHAR(255);

-- +goose Down
ALTER TABLE steam_users
    DROP COLUMN personaname,
    DROP COLUMN profileurl,
    DROP COLUMN avatar,
    DROP COLUMN avatar_medium,
    DROP COLUMN avatar_full;
