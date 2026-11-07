-- +goose Up
CREATE TABLE steam_users (
	id INTEGER PRIMARY KEY,
	steam_id BIGINT UNIQUE NOT NULL,
	name VARCHAR(255) NOT NULL
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
    release_coming_soon BOOLEAN
);

CREATE TABLE categories (
    id INT PRIMARY KEY,
    description VARCHAR(255)
);

CREATE TABLE app_categories (
    app_id INT,
    category_id INT,
    PRIMARY KEY (app_id, category_id),
    FOREIGN KEY (app_id) REFERENCES apps(steam_appid),
    FOREIGN KEY (category_id) REFERENCES categories(id)
);

CREATE TABLE genres (
    id INT PRIMARY KEY,
    description VARCHAR(255)
);

CREATE TABLE app_genres (
    app_id INT,
    genre_id INT,
    PRIMARY KEY (app_id, genre_id),
    FOREIGN KEY (app_id) REFERENCES apps(steam_appid),
    FOREIGN KEY (genre_id) REFERENCES genres(id)
);

CREATE TABLE steam_user_apps (
    steam_id BIGINT,
    app_id INT,
    PRIMARY KEY (steam_id, app_id),
    FOREIGN KEY (steam_id) REFERENCES steam_users(steam_id),
    FOREIGN KEY (app_id) REFERENCES apps(steam_appid)
);

CREATE TABLE app_alias (
    app_id INT NOT NULL,
    alias_id INT PRIMARY KEY NOT NULL,
    FOREIGN KEY (app_id) REFERENCES apps(steam_appid)
);

-- +goose Down
DROP TABLE app_alias;
DROP TABLE steam_user_apps;
DROP TABLE app_genres;
DROP TABLE app_categories;
DROP TABLE steam_users;
DROP TABLE genres;
DROP TABLE categories;
DROP TABLE apps;
