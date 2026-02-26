package smtp

import (
	"context"
	"sync/atomic"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/delivery"
	"github.com/sungwon/smtp-proxy/server/internal/logger"
	"github.com/sungwon/smtp-proxy/server/internal/msgstore"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// Backend implements the go-smtp Backend interface.
// It manages session creation and enforces connection limits.
type Backend struct {
	queries  storage.Querier
	delivery delivery.Service
	store    msgstore.MessageStore
	log      zerolog.Logger
	maxConns int
	active   atomic.Int64
}

// NewBackend creates a new SMTP backend with the given Querier, delivery service,
// logger, and maximum concurrent connection limit.
func NewBackend(queries storage.Querier, delivery delivery.Service, store msgstore.MessageStore, log zerolog.Logger, maxConns int) *Backend {
	return &Backend{
		queries:  queries,
		delivery: delivery,
		store:    store,
		log:      log,
		maxConns: maxConns,
	}
}

// NewSession is called after a client sends EHLO/HELO. It enforces connection
// limits and creates a new Session for the connection.
func (b *Backend) NewSession(conn *gosmtp.Conn) (gosmtp.Session, error) {
	current := b.active.Add(1)
	if int(current) > b.maxConns {
		b.active.Add(-1)
		b.log.Warn().
			Int64("active", current-1).
			Int("max", b.maxConns).
			Msg("connection limit reached")
		return nil, &gosmtp.SMTPError{
			Code:         421,
			EnhancedCode: gosmtp.EnhancedCode{4, 7, 0},
			Message:      "Too many connections",
		}
	}

	correlationID := logger.NewCorrelationID()
	ctx := context.Background()
	ctx = logger.WithCorrelationID(ctx, correlationID)

	sessionLog := b.log.With().
		Str("correlation_id", correlationID).
		Str("remote_addr", conn.Hostname()).
		Logger()

	sessionLog.Info().Msg("new SMTP session")

	return &Session{
		ctx:     ctx,
		queries: b.queries,
		log:     sessionLog,
		backend: b,
	}, nil
}

// ActiveSessions returns the current number of active SMTP sessions.
func (b *Backend) ActiveSessions() int64 {
	return b.active.Load()
}
