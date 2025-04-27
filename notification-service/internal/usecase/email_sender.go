package usecase

import (
	"log"
)

// DummyEmailSender заглушка для отправки email
type DummyEmailSender struct {
}

func NewDummyEmailSender() *DummyEmailSender {
	return &DummyEmailSender{}
}

// SendEmail отправляет email (в нашей заглушке просто логирует)
func (s *DummyEmailSender) SendEmail(to, subject, message string) error {
	log.Printf("Отправка email на %s с темой '%s': %s", to, subject, message)
	return nil
}

// SmtpEmailSender отправщик email через SMTP
type SmtpEmailSender struct {
	host     string
	port     string
	user     string
	password string
	from     string
}

func NewSmtpEmailSender(host, port, user, password, from string) *SmtpEmailSender {
	return &SmtpEmailSender{
		host:     host,
		port:     port,
		user:     user,
		password: password,
		from:     from,
	}
}

func (s *SmtpEmailSender) SendEmail(to, subject, message string) error {
	// В реальном приложении здесь была бы отправка через SMTP
	// Сейчас просто логируем
	log.Printf("[SMTP] Отправка email от %s на %s с темой '%s': %s", s.from, to, subject, message)
	return nil
}
