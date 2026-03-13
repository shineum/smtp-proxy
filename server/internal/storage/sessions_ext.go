package storage

import "context"

// UpdateSessionRefreshTokenParams contains the parameters for updating a session's refresh token hash.
type UpdateSessionRefreshTokenParams struct {
	ID               int32  `json:"id"`
	RefreshTokenHash string `json:"refresh_token_hash"`
}

// UpdateSessionRefreshToken updates the refresh_token_hash for an existing session.
// This is used after generating a refresh JWT that requires the session ID.
func (q *Queries) UpdateSessionRefreshToken(ctx context.Context, arg UpdateSessionRefreshTokenParams) error {
	const query = `UPDATE sessions SET refresh_token_hash = $1 WHERE id = $2`
	_, err := q.db.Exec(ctx, query, arg.RefreshTokenHash, arg.ID)
	return err
}
