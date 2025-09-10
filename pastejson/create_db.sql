-- psql -h 127.0.0.1 -p 5432 -U your_username -f create_db.sql

-- create_db.sql 文件内容
CREATE DATABASE pastejson;
-- 假设你要将所有权限授予一个已存在的用户 'user'
-- CREATE USER "lumin" WITH PASSWORD 'your_password_here'; -- 请替换 'your_password_here' 为强密码
GRANT ALL PRIVILEGES ON DATABASE pastejson TO "lumin";
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO "lumin";
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO "lumin";
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO "lumin";
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, UPDATE ON TABLES TO "lumin";

