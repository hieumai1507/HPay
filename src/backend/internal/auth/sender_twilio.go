package auth

// Twilio adapter behind the Sender port (ADR-0017). It sends the code HPay generated (HPay
// keeps generating, hashing, counting and expiring codes; the provider only delivers them) with
// Twilio's Programmable Messaging API: POST /2010-04-01/Accounts/{sid}/Messages.json, form
// encoded, HTTP Basic auth with an API key. The HTTP client is the guarded one from the gateway
// package (https only, public addresses only, no redirects, C-10). The code and the API secret
// are never logged.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/gateway"
)

// TwilioConfig holds the account settings. APISecret must never be logged.
type TwilioConfig struct {
	AccountSID string
	APIKey     string
	APISecret  string
	From       string // E.164 sender number or messaging service number
	BaseURL    string // default https://api.twilio.com
}

func (c TwilioConfig) complete() bool {
	return c.AccountSID != "" && c.APIKey != "" && c.APISecret != "" && c.From != ""
}

// PhoneLookup returns the destination phone number of a user.
type PhoneLookup func(ctx context.Context, userID uuid.UUID) (string, error)

type twilioSender struct {
	cfg    TwilioConfig
	log    *slog.Logger
	client *gateway.FormClient
	phone  PhoneLookup
}

// NewTwilioSender builds the production adapter with the guarded client.
func NewTwilioSender(cfg TwilioConfig, log *slog.Logger, phone PhoneLookup) Sender {
	return &twilioSender{cfg: cfg, log: log, client: gateway.NewFormClient(10 * time.Second), phone: phone}
}

// newTwilioSenderForTest swaps the client for one that can reach a local TLS test server.
func newTwilioSenderForTest(cfg TwilioConfig, log *slog.Logger, client *gateway.FormClient, phone PhoneLookup) Sender {
	return &twilioSender{cfg: cfg, log: log, client: client, phone: phone}
}

func (s *twilioSender) endpoint() string {
	base := strings.TrimRight(s.cfg.BaseURL, "/")
	if base == "" {
		base = "https://api.twilio.com"
	}
	return base + "/2010-04-01/Accounts/" + url.PathEscape(s.cfg.AccountSID) + "/Messages.json"
}

func (s *twilioSender) deliver(ctx context.Context, userID uuid.UUID, body string) error {
	phone, err := s.phone(ctx, userID)
	if err != nil {
		return err
	}
	to, err := toE164(phone)
	if err != nil {
		return err
	}
	_, err = s.client.PostForm(ctx, s.endpoint(), s.cfg.APIKey, s.cfg.APISecret,
		map[string]string{"To": to, "From": s.cfg.From, "Body": body})
	return err
}

func (s *twilioSender) SendCode(ctx context.Context, userID uuid.UUID, purpose, code string) error {
	if err := s.deliver(ctx, userID, fmt.Sprintf("HPay: your code is %s. It expires in 2 minutes. Never share it.", code)); err != nil {
		s.log.Warn("code delivery failed", "user", userID, "purpose", purpose, "err", err)
		return err
	}
	s.log.Info("one-time code sent", "user", userID, "purpose", purpose) // never the code
	return nil
}

func (s *twilioSender) Notify(ctx context.Context, userID uuid.UUID, n Notice) error {
	if err := s.deliver(ctx, userID, "HPay: "+n.Text); err != nil {
		s.log.Warn("notice delivery failed", "user", userID, "kind", n.Kind, "err", err)
		return err
	}
	return nil
}

// toE164 turns a stored Vietnamese phone (0901234567, 84901234567 or +84901234567) into E.164.
func toE164(phone string) (string, error) {
	p := strings.ReplaceAll(strings.TrimSpace(phone), " ", "")
	switch {
	case strings.HasPrefix(p, "+"):
		p = p[1:]
	case strings.HasPrefix(p, "0"):
		p = "84" + p[1:]
	}
	if len(p) < 9 || len(p) > 15 {
		return "", errors.New("phone number is not usable for delivery")
	}
	for _, r := range p {
		if r < '0' || r > '9' {
			return "", errors.New("phone number is not usable for delivery")
		}
	}
	return "+" + p, nil
}

// SenderConfig selects the sender for an environment.
type SenderConfig struct {
	Env      string
	Provider string // "" or "twilio"
	Twilio   TwilioConfig
}

// NewSenderFromConfig picks the sender: an explicit provider is honoured in every environment
// and refuses (never falls back to a log-only sender) when its settings are incomplete; with
// no provider, production and demo refuse and the other environments log only.
func NewSenderFromConfig(c SenderConfig, log *slog.Logger, phone PhoneLookup) Sender {
	if c.Provider == "twilio" {
		if !c.Twilio.complete() {
			return refusingSender{}
		}
		return NewTwilioSender(c.Twilio, log, phone)
	}
	return NewSender(c.Env, log)
}

// UseSender replaces the service's delivery channel (wiring in main and the router).
func (s *Service) UseSender(sender Sender) { s.sender = sender }

// PhoneOf returns the phone lookup backed by the database, for the Twilio adapter.
func (s *Service) PhoneOf() PhoneLookup {
	return func(ctx context.Context, userID uuid.UUID) (string, error) {
		return s.q.GetUserPhone(ctx, userID)
	}
}

// Wire applies the configuration to a Service: the TOTP encryption key and the code delivery
// channel. Used by the server and the router so both behave the same.
func (s *Service) Wire(totpKey []byte, c SenderConfig) {
	s.SetTOTPKey(totpKey)
	s.UseSender(NewSenderFromConfig(c, s.log, s.PhoneOf()))
}
