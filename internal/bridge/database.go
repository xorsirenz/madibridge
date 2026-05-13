package bridge

func (b *Bridge) ensureTables() error {
	_, err := b.db.Exec(`
		CREATE TABLE IF NOT EXISTS message_map (
			discord_id TEXT UNIQUE,
			matrix_id TEXT UNIQUE,
			discord_webhook_msg_id TEXT,
			discord_username TEXT
		);

		CREATE TABLE IF NOT EXISTS bridge_state (
			key TEXT PRIMARY KEY,
			value TEXT
		);
	`)

	return err
}

func (b *Bridge) storeMessageMap(m MessageMap) error {
	_, err := b.db.Exec(`
		INSERT INTO message_map(
			discord_id,
			matrix_id,
			discord_webhook_msg_id,
			discord_username
		)
		VALUES($1, $2, $3, $4)
		ON CONFLICT DO NOTHING
	`,
		m.DiscordID,
		m.MatrixID,
		m.DiscordWebhookMsgID,
		m.Username,
	)

	return err
}

func (b *Bridge) getMatrixID(discordID string) (string, error) {
	var matrixID string

	err := b.db.QueryRow(`
		SELECT matrix_id
		FROM message_map
		WHERE discord_id=$1
	`, discordID).Scan(&matrixID)

	return matrixID, err
}

func (b *Bridge) getDiscordID(matrixID string) (string, error) {
	var discordID string

	err := b.db.QueryRow(`
		SELECT discord_id
		FROM message_map
		WHERE matrix_id=$1
	`, matrixID).Scan(&discordID)

	return discordID, err
}

func (b *Bridge) getDiscordWebhookID(matrixID string) (string, error) {
	var webhookID string

	err := b.db.QueryRow(`
		SELECT discord_webhook_msg_id
		FROM message_map
		WHERE matrix_id=$1
	`, matrixID).Scan(&webhookID)

	return webhookID, err
}

func (b *Bridge) getDiscordUsername(discordID string) (string, error) {
	var username string

	err := b.db.QueryRow(`
		SELECT discord_username
		FROM message_map
		WHERE discord_id=$1
	`, discordID).Scan(&username)

	return username, err
}
