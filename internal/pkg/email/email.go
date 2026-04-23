package email

import (
	"fmt"
	"net/smtp"
	"strings"

	"github.com/rs/zerolog/log"
)

// Mailer sends plain-text emails over SMTP.
// Email is best-effort — failures are logged but never bubble up to the caller.
type Mailer struct {
	host      string
	port      int
	user      string
	pass      string
	fromAddr  string
	enabled   bool // false when SMTP credentials are not configured
}

// New creates a Mailer. If host is empty, the mailer runs in no-op mode
// (silently skips sends). This allows the backend to run without email config.
func New(host string, port int, user, pass, fromAddr string) *Mailer {
	return &Mailer{
		host:     host,
		port:     port,
		user:     user,
		pass:     pass,
		fromAddr: fromAddr,
		enabled:  host != "" && user != "" && pass != "",
	}
}

// Send dispatches a plain-text email. Non-blocking — always returns immediately.
// If SMTP is not configured, the send is silently skipped.
func (m *Mailer) Send(to, subject, body string) {
	if !m.enabled {
		log.Debug().Str("to", to).Str("subject", subject).Msg("email skipped (SMTP not configured)")
		return
	}

	go func() {
		if err := m.send(to, subject, body); err != nil {
			log.Error().Err(err).Str("to", to).Str("subject", subject).Msg("email send failed")
		}
	}()
}

func (m *Mailer) send(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", m.host, m.port)
	auth := smtp.PlainAuth("", m.user, m.pass, m.host)

	msg := strings.Join([]string{
		fmt.Sprintf("From: BachelorPad <%s>", m.fromAddr),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	return smtp.SendMail(addr, auth, m.fromAddr, []string{to}, []byte(msg))
}
