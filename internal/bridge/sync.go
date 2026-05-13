package bridge

func (b *Bridge) getSyncToken() string {
	var token string

	err := b.db.QueryRow(`
		SELECT value
		FROM bridge_state
		WHERE key='sync_token'
	`).Scan(&token)

	if err != nil {
		return ""
	}

	return token
}

func (b *Bridge) setSyncToken(token string) error {
	_, err := b.db.Exec(`
		INSERT INTO bridge_state(key, value)
		VALUES('sync_token', $1)
		ON CONFLICT(key)
		DO UPDATE SET value=EXCLUDED.value
	`, token)

	return err
}
