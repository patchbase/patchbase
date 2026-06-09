package mailer

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/smtp"
	"sort"
	"strings"

	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/sql"
)

//go:embed templates/*
var templateFS embed.FS

type mailer struct {
	settings  services.Settings
	queries   sql.Querier
	injector  do.Injector
	templates map[string]*template.Template
}

func NewMailer(i do.Injector) (Mailer, error) {
	settings, err := do.Invoke[services.Settings](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get Settings: %w", err)
	}
	queries, err := do.Invoke[sql.Querier](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.Querier: %w", err)
	}

	htmlTmpl, err := template.ParseFS(templateFS, "templates/report.html.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse html template: %w", err)
	}
	txtTmpl, err := template.ParseFS(templateFS, "templates/report.txt.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse txt template: %w", err)
	}

	templates := map[string]*template.Template{
		"report.html": htmlTmpl,
		"report.txt":  txtTmpl,
	}

	return &mailer{
		settings:  settings,
		queries:   queries,
		injector:  i,
		templates: templates,
	}, nil
}

func (m *mailer) TestConnection(ctx context.Context, s services.SMTPSettings, to string) error {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	auth := smtp.PlainAuth("", s.Username, s.Password, s.Host)

	msg := []byte("To: " + to + "\r\n" +
		"Subject: Test Email from PatchBase\r\n" +
		"\r\n" +
		"This is a test email to verify your SMTP settings.\r\n")

	err := smtp.SendMail(addr, auth, s.From, []string{to}, msg)
	if err != nil {
		return fmt.Errorf("failed to send test email: %w", err)
	}
	return nil
}

type ReportData struct {
	TotalHosts    int
	WithCritical  int
	WithImportant int
	WithMedium    int
	TopHosts      []services.HostInfo
}

func (m *mailer) SendReport(ctx context.Context, to []string) error {
	s, err := m.settings.GetSMTPSettings(ctx)
	if err != nil {
		return fmt.Errorf("failed to get smtp settings: %w", err)
	}
	if s.Host == "" {
		return fmt.Errorf("smtp settings not configured")
	}

	if len(to) == 0 {
		admins, err := m.queries.ListAdminUsers(ctx)
		if err != nil {
			return fmt.Errorf("failed to list admin users: %w", err)
		}
		for _, admin := range admins {
			to = append(to, admin.Email)
		}
	}
	if len(to) == 0 {
		return nil // No admins to send to
	}

	hostsSvc, err := do.Invoke[services.Hosts](m.injector)
	if err != nil {
		return fmt.Errorf("failed to get Hosts service: %w", err)
	}
	allHosts, err := hostsSvc.ListHosts(ctx)
	if err != nil {
		return fmt.Errorf("failed to list hosts: %w", err)
	}

	data := ReportData{
		TotalHosts:    0,
		WithCritical:  0,
		WithImportant: 0,
		WithMedium:    0,
		TopHosts:      nil,
	}
	for _, h := range allHosts {
		data.TotalHosts++
		if h.CriticalCount > 0 {
			data.WithCritical++
		}
		if h.ImportantCount > 0 {
			data.WithImportant++
		}
		if h.ModerateCount > 0 {
			data.WithMedium++
		}
	}

	sort.Slice(allHosts, func(i, j int) bool {
		a, b := allHosts[i], allHosts[j]
		if a.CriticalCount != b.CriticalCount {
			return a.CriticalCount > b.CriticalCount
		}
		if a.ImportantCount != b.ImportantCount {
			return a.ImportantCount > b.ImportantCount
		}
		return a.ModerateCount > b.ModerateCount
	})

	data.TopHosts = allHosts
	if len(data.TopHosts) > 5 {
		data.TopHosts = data.TopHosts[:5]
	}

	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	auth := smtp.PlainAuth("", s.Username, s.Password, s.Host)

	boundary := "patchbase-boundary-" + fmt.Sprintf("%d", 123456789)

	var textBuf bytes.Buffer
	if err := m.templates["report.txt"].Execute(&textBuf, data); err != nil {
		return fmt.Errorf("failed to execute txt template: %w", err)
	}

	var htmlBuf bytes.Buffer
	if err := m.templates["report.html"].Execute(&htmlBuf, data); err != nil {
		return fmt.Errorf("failed to execute html template: %w", err)
	}

	var msgBuf bytes.Buffer
	fmt.Fprintf(&msgBuf, "To: %s\r\n", strings.Join(to, ", "))
	msgBuf.WriteString("Subject: PatchBase Daily Report\r\n")
	msgBuf.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&msgBuf, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary)
	msgBuf.WriteString("\r\n")

	// Text part
	fmt.Fprintf(&msgBuf, "--%s\r\n", boundary)
	msgBuf.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	msgBuf.WriteString("\r\n")
	msgBuf.WriteString(textBuf.String())
	msgBuf.WriteString("\r\n\r\n")

	// HTML part
	fmt.Fprintf(&msgBuf, "--%s\r\n", boundary)
	msgBuf.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	msgBuf.WriteString("\r\n")
	msgBuf.WriteString(htmlBuf.String())
	msgBuf.WriteString("\r\n\r\n")

	fmt.Fprintf(&msgBuf, "--%s--\r\n", boundary)

	err = smtp.SendMail(addr, auth, s.From, to, msgBuf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send report email: %w", err)
	}
	return nil
}
