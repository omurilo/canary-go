-- Migration: Add player_outfits table for outfit storage
-- Date: 2026-07-25
-- System: Mounts & Outfits (B1)

CREATE TABLE IF NOT EXISTS `player_outfits` (
    `player_id` int(11) NOT NULL,
    `looktype` int(11) NOT NULL,
    `addons` tinyint(4) NOT NULL DEFAULT '0',
    PRIMARY KEY (`player_id`, `looktype`),
    CONSTRAINT `player_outfits_players_fk`
        FOREIGN KEY (`player_id`)
        REFERENCES `players` (`id`)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Note: player_mounts table already exists in the schema
-- Mounts are also stored using storage keys 10002001-10002011 (bitflags)
