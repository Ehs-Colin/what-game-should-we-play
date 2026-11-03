-- +goose Up
CREATE TABLE steam_users (
	id INTEGER PRIMARY KEY,
	steam_id BIGINT UNIQUE NOT NULL,
	name VARCHAR(255) NOT NULL
);

-- +goose Down
DROP TABLE steam_users;
