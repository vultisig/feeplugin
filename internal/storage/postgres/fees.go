package postgres

import "context"

func (p *PostgresBackend) GetPublicKeys(ctx context.Context) ([]string, error) {
	query := `SELECT public_key FROM plugin_keys` // ordered by creation time for consistency

	var publicKeys []string

	rows, err := p.pool.Query(ctx, query)
	if err != nil {
		return nil, err // propagate database query error
	}
	defer rows.Close() // ensure rows are closed to release resources

	for rows.Next() {
		var publicKey string
		if err := rows.Scan(&publicKey); err != nil {
			return nil, err // handle scan error for individual rows
		}
		publicKeys = append(publicKeys, publicKey)
	}

	if err := rows.Err(); err != nil {
		return nil, err // check for any iteration errors
	}

	return publicKeys, nil
}

func (p *PostgresBackend) InsertPublicKey(ctx context.Context, publicKey string) error {
	query := `INSERT INTO plugin_keys (public_key) VALUES ($1) ON CONFLICT (public_key) DO NOTHING`

	_, err := p.pool.Exec(ctx, query, publicKey)
	if err != nil {
		return err
	}

	return nil
}
