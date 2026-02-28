// Package bootstrap provides startup-time initialization routines
// such as seeding the system admin account.
package bootstrap

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// SeedSystemAdmin ensures a system group and admin user exist.
// It is idempotent: if the system group already has members, it returns immediately.
//
// Requirements:
//   - REQ-AUTH-001: Auto-create system group + admin on first boot
//   - REQ-AUTH-002: Update admin password when SMTP_PROXY_ADMIN_PASSWORD is set
//   - REQ-AUTH-003: Idempotent (safe on every startup)
func SeedSystemAdmin(ctx context.Context, queries storage.Querier, log zerolog.Logger, email, password string) error {
	// Step 1: Check if system group already exists.
	group, err := queries.GetGroupByName(ctx, "system")
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	if err == nil {
		// System group exists; check if it has members.
		members, err := queries.ListGroupMembersByGroupID(ctx, group.ID)
		if err != nil {
			return err
		}
		if len(members) > 0 {
			log.Info().Msg("system admin already exists, skipping seed")

			// REQ-AUTH-002: Update password if provided.
			if password != "" {
				return updateAdminPassword(ctx, queries, log, email, password)
			}
			return nil
		}
	}

	// Step 2: Create system group if it doesn't exist.
	if errors.Is(err, pgx.ErrNoRows) {
		group, err = queries.CreateGroup(ctx, storage.CreateGroupParams{
			Name:      "system",
			GroupType: "system",
		})
		if err != nil {
			return err
		}
		log.Info().Str("group_id", group.ID.String()).Msg("system group created")
	}

	// Step 3: Create or get the admin user.
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	user, err := queries.CreateUser(ctx, storage.CreateUserParams{
		Email:        email,
		PasswordHash: passwordHash,
		AccountType:  "user",
	})
	if err != nil {
		// User may already exist by email; try to fetch.
		existingUser, getErr := queries.GetUserByEmail(ctx, email)
		if getErr != nil {
			return err // return original creation error
		}
		user = existingUser
		log.Info().Str("email", email).Msg("admin user already exists, reusing")
	} else {
		log.Info().Str("user_id", user.ID.String()).Str("email", email).Msg("admin user created")
	}

	// Step 4: Add admin as owner of the system group.
	_, err = queries.CreateGroupMember(ctx, storage.CreateGroupMemberParams{
		GroupID: group.ID,
		UserID:  user.ID,
		Role:    "owner",
	})
	if err != nil {
		return err
	}

	log.Info().
		Str("email", email).
		Str("group", "system").
		Str("role", "owner").
		Msg("system admin seeded successfully")

	return nil
}

// updateAdminPassword hashes and updates the admin user's password.
func updateAdminPassword(ctx context.Context, queries storage.Querier, log zerolog.Logger, email, password string) error {
	user, err := queries.GetUserByEmail(ctx, email)
	if err != nil {
		return nil // admin email not found; nothing to update
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	if err := queries.UpdateUserPassword(ctx, storage.UpdateUserPasswordParams{
		ID:           user.ID,
		PasswordHash: hash,
	}); err != nil {
		return err
	}

	log.Info().Str("email", email).Msg("admin password updated from environment")
	return nil
}
