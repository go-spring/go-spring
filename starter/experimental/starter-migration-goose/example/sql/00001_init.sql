-- +goose Up
CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT);

-- +goose Down
DROP TABLE widgets;
