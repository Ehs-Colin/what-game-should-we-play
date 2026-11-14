-- +goose Up
CREATE TABLE steam_users (
	steam_id BIGINT PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	wishlist_private BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE apps (
    steam_appid INT PRIMARY KEY,
    name VARCHAR(255),
    detailed_description TEXT,
    about_the_game TEXT,
    short_description TEXT,
    header_image VARCHAR(255),
    website VARCHAR(255),
    price_currency VARCHAR(10),
    price_initial INT,
    price_final INT,
    price_discount_percent INT,
    price_final_formatted VARCHAR(255),
    platform_windows BOOLEAN,
    platform_mac BOOLEAN,
    platform_linux BOOLEAN,
    release_date VARCHAR(255),
    release_coming_soon BOOLEAN,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE categories (
    id INT PRIMARY KEY,
    description VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE app_categories (
    app_id INT,
    category_id INT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (app_id, category_id),
    FOREIGN KEY (app_id) REFERENCES apps(steam_appid),
    FOREIGN KEY (category_id) REFERENCES categories(id)
);

CREATE TABLE genres (
    id INT PRIMARY KEY,
    description VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE app_genres (
    app_id INT,
    genre_id INT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (app_id, genre_id),
    FOREIGN KEY (app_id) REFERENCES apps(steam_appid),
    FOREIGN KEY (genre_id) REFERENCES genres(id)
);

-- Every app has a self-alias (80 -> 80) plus one row per other ID that
-- Steam resolves to it (100 -> 80).
CREATE TABLE app_alias (
    app_id INT NOT NULL,
    alias_id INT PRIMARY KEY NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    FOREIGN KEY (app_id) REFERENCES apps(steam_appid)
);

-- app_id is the ID the user owns, looked up through app_alias.
CREATE TABLE steam_user_apps (
    steam_id BIGINT,
    app_id INT,
    removed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (steam_id, app_id),
    FOREIGN KEY (steam_id) REFERENCES steam_users(steam_id),
    FOREIGN KEY (app_id) REFERENCES app_alias(alias_id)
);

-- app_id is the ID returned by Steam, looked up through app_alias.
CREATE TABLE steam_user_wishlist (
    steam_id BIGINT NOT NULL,
    app_id INT NOT NULL,
    date_added TIMESTAMPTZ,
    removed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (steam_id, app_id),
    FOREIGN KEY (steam_id) REFERENCES steam_users(steam_id),
    FOREIGN KEY (app_id) REFERENCES app_alias(alias_id)
);

-- One row per normalized Steam resource. response_body is the raw HTTP body.
CREATE TABLE steam_api_cache (
    cache_key TEXT PRIMARY KEY,
    response_body BYTEA NOT NULL,
    fetched_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    call_count INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX steam_api_cache_expires_at ON steam_api_cache (expires_at);

-- One row per successful cache miss. Old rows are pruned per key on write and
-- globally at startup.
CREATE TABLE steam_api_cache_calls (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cache_key TEXT NOT NULL,
    fetched_at TIMESTAMPTZ NOT NULL,
    FOREIGN KEY (cache_key) REFERENCES steam_api_cache(cache_key) ON DELETE CASCADE
);
CREATE INDEX steam_api_cache_calls_key_fetched_at
    ON steam_api_cache_calls (cache_key, fetched_at);

CREATE TABLE import_jobs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    steam_id BIGINT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    total_apps INT NOT NULL DEFAULT 0,
    processed_apps INT NOT NULL DEFAULT 0,
    skipped_apps INT NOT NULL DEFAULT 0,   -- not found on Steam
    failed_apps INT NOT NULL DEFAULT 0,
    wishlist_total_apps INT NOT NULL DEFAULT 0,
    wishlist_processed_apps INT NOT NULL DEFAULT 0,
    wishlist_skipped_apps INT NOT NULL DEFAULT 0,
    wishlist_failed_apps INT NOT NULL DEFAULT 0,
    error TEXT,                            -- set when the whole job fails
    created_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    FOREIGN KEY (steam_id) REFERENCES steam_users(steam_id)
);
CREATE INDEX import_jobs_steam_id_status ON import_jobs (steam_id, status);

-- +goose Down
DROP TABLE steam_api_cache_calls;
DROP TABLE steam_api_cache;
DROP TABLE import_jobs;
DROP TABLE steam_user_wishlist;
DROP TABLE steam_user_apps;
DROP TABLE app_alias;
DROP TABLE app_genres;
DROP TABLE app_categories;
DROP TABLE steam_users;
DROP TABLE genres;
DROP TABLE categories;
DROP TABLE apps;
