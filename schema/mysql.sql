-- Canary-Go MySQL/MariaDB schema.
--
-- In the docker stack the MariaDB container is initialised with the full
-- canonical Canary schema (../schema.sql), so these CREATE TABLE IF NOT EXISTS
-- statements are skipped for tables that already exist. They also let canary-go
-- run standalone against a bare MariaDB. The async_jobs table is canary-go's
-- own (MyAAC/login-server don't use it).

CREATE TABLE IF NOT EXISTS accounts (
    id                 INT UNSIGNED NOT NULL AUTO_INCREMENT,
    name               VARCHAR(32) NOT NULL,
    password           VARCHAR(255) NOT NULL,
    email              VARCHAR(255) NOT NULL DEFAULT '',
    premdays           INT NOT NULL DEFAULT 0,
    premdays_purchased INT NOT NULL DEFAULT 0,
    lastday            INT UNSIGNED NOT NULL DEFAULT 0,
    type               TINYINT UNSIGNED NOT NULL DEFAULT 1,
    coins              INT UNSIGNED NOT NULL DEFAULT 0,
    coins_transferable INT UNSIGNED NOT NULL DEFAULT 0,
    tournament_coins   INT UNSIGNED NOT NULL DEFAULT 0,
    creation           INT UNSIGNED NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE KEY name (name),
    KEY email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS players (
    id            INT NOT NULL AUTO_INCREMENT,
    name          VARCHAR(255) NOT NULL,
    group_id      INT NOT NULL DEFAULT 1,
    account_id    INT UNSIGNED NOT NULL DEFAULT 0,
    level         INT NOT NULL DEFAULT 1,
    vocation      INT NOT NULL DEFAULT 0,
    health        INT NOT NULL DEFAULT 150,
    healthmax     INT NOT NULL DEFAULT 150,
    experience    BIGINT NOT NULL DEFAULT 0,
    lookbody      INT NOT NULL DEFAULT 0,
    lookfeet      INT NOT NULL DEFAULT 0,
    lookhead      INT NOT NULL DEFAULT 0,
    looklegs      INT NOT NULL DEFAULT 0,
    looktype      INT NOT NULL DEFAULT 136,
    lookaddons    INT NOT NULL DEFAULT 0,
    lookmount     INT NOT NULL DEFAULT 0,
    maglevel      INT NOT NULL DEFAULT 0,
    mana          INT NOT NULL DEFAULT 0,
    manamax       INT NOT NULL DEFAULT 0,
    manaspent     BIGINT UNSIGNED NOT NULL DEFAULT 0,
    soul          INT UNSIGNED NOT NULL DEFAULT 0,
    town_id       INT NOT NULL DEFAULT 1,
    posx          INT NOT NULL DEFAULT 0,
    posy          INT NOT NULL DEFAULT 0,
    posz          INT NOT NULL DEFAULT 0,
    conditions    BLOB,
    cap           INT NOT NULL DEFAULT 400,
    sex           INT NOT NULL DEFAULT 0,
    lastlogin     BIGINT UNSIGNED NOT NULL DEFAULT 0,
    lastip        INT UNSIGNED NOT NULL DEFAULT 0,
    save          TINYINT NOT NULL DEFAULT 1,
    lastlogout    BIGINT UNSIGNED NOT NULL DEFAULT 0,
    balance       BIGINT UNSIGNED NOT NULL DEFAULT 0,
    stamina       SMALLINT UNSIGNED NOT NULL DEFAULT 2520,
    skill_fist            INT UNSIGNED NOT NULL DEFAULT 10,
    skill_fist_tries      BIGINT UNSIGNED NOT NULL DEFAULT 0,
    skill_club            INT UNSIGNED NOT NULL DEFAULT 10,
    skill_club_tries      BIGINT UNSIGNED NOT NULL DEFAULT 0,
    skill_sword           INT UNSIGNED NOT NULL DEFAULT 10,
    skill_sword_tries     BIGINT UNSIGNED NOT NULL DEFAULT 0,
    skill_axe             INT UNSIGNED NOT NULL DEFAULT 10,
    skill_axe_tries       BIGINT UNSIGNED NOT NULL DEFAULT 0,
    skill_dist            INT UNSIGNED NOT NULL DEFAULT 10,
    skill_dist_tries      BIGINT UNSIGNED NOT NULL DEFAULT 0,
    skill_shielding       INT UNSIGNED NOT NULL DEFAULT 10,
    skill_shielding_tries BIGINT UNSIGNED NOT NULL DEFAULT 0,
    skill_fishing         INT UNSIGNED NOT NULL DEFAULT 10,
    skill_fishing_tries   BIGINT UNSIGNED NOT NULL DEFAULT 0,
    deletion      BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE KEY name (name),
    KEY account_id (account_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS player_storage (
    player_id INT NOT NULL,
    `key`     INT NOT NULL DEFAULT 0,
    value     INT NOT NULL DEFAULT 0,
    PRIMARY KEY (player_id, `key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS player_mounts (
    player_id INT NOT NULL,
    mount_id  INT NOT NULL,
    PRIMARY KEY (player_id, mount_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS global_storage (
    `key` INT NOT NULL,
    value INT NOT NULL DEFAULT 0,
    PRIMARY KEY (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS player_items (
    player_id  INT NOT NULL DEFAULT 0,
    pid        INT NOT NULL DEFAULT 0,
    sid        INT NOT NULL DEFAULT 0,
    itemtype   SMALLINT UNSIGNED NOT NULL DEFAULT 0,
    count      SMALLINT NOT NULL DEFAULT 0,
    attributes BLOB,
    KEY player_id (player_id),
    UNIQUE KEY player_sid (player_id, sid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS player_spells (
    player_id INT NOT NULL,
    name      VARCHAR(255) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS players_online (
    player_id INT NOT NULL,
    PRIMARY KEY (player_id)
) ENGINE=MEMORY;

CREATE TABLE IF NOT EXISTS towns (
    id   INT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(255) NOT NULL,
    posx INT NOT NULL DEFAULT 0,
    posy INT NOT NULL DEFAULT 0,
    posz INT NOT NULL DEFAULT 0,
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS server_config (
    config VARCHAR(50) NOT NULL,
    value  VARCHAR(256) NOT NULL DEFAULT '',
    PRIMARY KEY (config)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- canary-go async job queue (table-polled with FOR UPDATE SKIP LOCKED).
CREATE TABLE IF NOT EXISTS async_jobs (
    id         BIGINT NOT NULL AUTO_INCREMENT,
    kind       VARCHAR(64) NOT NULL,
    payload    JSON NOT NULL,
    status     VARCHAR(16) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO towns (id, name, posx, posy, posz) VALUES (1, 'Thais', 1000, 1000, 7);
INSERT IGNORE INTO server_config (config, value) VALUES ('db_version', '1');
