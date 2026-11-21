package auth

// Delivery port for one-time codes and security notices (ADR-0016, C-09, C-11).
//
// HPay has no SMS or e-mail provider yet. The port lets the reset flow be built and tested
// now; a provider is chosen in a separate ADR together with the 2FA decision. Until then:
//   - dev and ci use a sender that logs only that a message was sent (never the code),
//   - production uses a sender that refuses, so a reset cannot silently "succeed" without
//     reaching the user (fail closed, C-13).

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
)

// Purposes of one-time codes.
const PurposePINReset = "pin_reset"

// NoticeKind names a security notice sent to the account holder.
type NoticeKind string

const NoticePINReset NoticeKind = "pin_reset"

// Notice is a message to the user about a security event. Text never contains a secret.
type Notice struct {
	Kind NoticeKind
	Text string
}

// ErrSenderUnavailable means no real delivery channel is configured.
var ErrSenderUnavailable = errors.New("code delivery is not configured")

// Sender delivers codes and notices to a user.
type Sender interface {
	SendCode(ctx context.Context, userID uuid.UUID, purpose, code string) error
	Notify(ctx context.Context, userID uuid.UUID, n Notice) error
}

// NewSender picks the sender for an environment: a log-only one outside production, a
// refusing one in production.
func NewSender(env string, log *slog.Logger) Sender {
	if env == "production" || env == "demo" {
		return refusingSender{}
	}
	return logSender{log: log}
}

type logSender struct{ log *slog.Logger }

func (s logSender) SendCode(_ context.Context, userID uuid.UUID, purpose, _ string) error {
	s.log.Info("one-time code sent (log-only sender)", "user", userID, "purpose", purpose)
	return nil
}
func (s logSender) Notify(_ context.Context, userID uuid.UUID, n Notice) error {
	s.log.Info("security notice sent (log-only sender)", "user", userID, "kind", n.Kind)
	return nil
}

type refusingSender struct{}

func (refusingSender) SendCode(context.Context, uuid.UUID, string, string) error {
	return ErrSenderUnavailable
}
func (refusingSender) Notify(context.Context, uuid.UUID, Notice) error { return ErrSenderUnavailable }
