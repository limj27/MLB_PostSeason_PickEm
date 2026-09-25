-- MLB Postseason Pick'em schema

CREATE TABLE IF NOT EXISTS users (
  id INT AUTO_INCREMENT PRIMARY KEY,
  username VARCHAR(50) UNIQUE NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  display_name VARCHAR(100) NOT NULL,
  is_admin BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
  token VARCHAR(64) PRIMARY KEY,
  user_id INT NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS rounds (
  id INT AUTO_INCREMENT PRIMARY KEY,
  round_key VARCHAR(20) UNIQUE NOT NULL,
  round_name VARCHAR(100) NOT NULL,
  best_of INT NOT NULL DEFAULT 7,
  has_mvp BOOLEAN NOT NULL DEFAULT FALSE,
  team_a VARCHAR(100),
  team_a_mlb_id INT,
  team_b VARCHAR(100),
  team_b_mlb_id INT,
  lock_time DATETIME NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, open, locked, final
  actual_winner VARCHAR(100),
  actual_length INT,
  actual_mvp VARCHAR(100),
  sort_order INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS picks (
  id INT AUTO_INCREMENT PRIMARY KEY,
  user_id INT NOT NULL,
  round_id INT NOT NULL,
  picked_team VARCHAR(100) NOT NULL,
  picked_length INT NOT NULL,
  picked_mvp VARCHAR(100),
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uniq_user_round (user_id, round_id),
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (round_id) REFERENCES rounds(id) ON DELETE CASCADE
);

-- Seed the standard bracket. Edit team_a/team_b once matchups are set,
-- or add more rows (e.g. Wild Card series) from the admin page.
INSERT IGNORE INTO rounds (round_key, round_name, best_of, has_mvp, sort_order) VALUES
  ('ALDS1', 'ALDS Game 1 Matchup', 5, FALSE, 10),
  ('ALDS2', 'ALDS Game 2 Matchup', 5, FALSE, 20),
  ('NLDS1', 'NLDS Game 1 Matchup', 5, FALSE, 30),
  ('NLDS2', 'NLDS Game 2 Matchup', 5, FALSE, 40),
  ('ALCS',  'ALCS',                7, TRUE,  50),
  ('NLCS',  'NLCS',                7, TRUE,  60),
  ('WS',    'World Series',        7, TRUE,  70);
