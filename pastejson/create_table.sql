-- psql -h 127.0.0.1 -p 5432 -U your_username -f create_table.sql
\c pastejson;

CREATE TABLE json_data (
    id BIGINT,
    data JSONB,

    PRIMARY KEY (id)
);

CREATE TABLE previous_relations (
    previous_id BIGINT REFERENCES json_data(id) ON DELETE CASCADE,
    id BIGINT REFERENCES json_data(id) ON DELETE CASCADE,
    -- 根据实际需求添加其他字段，如:
    -- source_id BIGINT REFERENCES json_data(id),

    PRIMARY KEY (previous_id, id) -- 联合主键确保唯一性
);

CREATE TABLE json_tags (
    id BIGINT REFERENCES json_data(id) ON DELETE CASCADE,
    tag VARCHAR(255),
    PRIMARY KEY (id, tag) -- 联合主键确保唯一性
);
