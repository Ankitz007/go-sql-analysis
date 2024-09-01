CREATE TABLE IF NOT EXISTS funds (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    fund_house VARCHAR(255),
    scheme_type VARCHAR(255),
    scheme_category VARCHAR(255),
    scheme_code INT UNIQUE,
    scheme_name VARCHAR(255)
);
CREATE TABLE IF NOT EXISTS nav_records (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    fund_id BIGINT,
    date DATE,
    nav FLOAT,
    FOREIGN KEY (fund_id) REFERENCES funds (id)
);