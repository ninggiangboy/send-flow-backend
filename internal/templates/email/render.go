package templates

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed *.go.tmpl
var templateFS embed.FS

var tmpls *template.Template

func init() {
	tmpls = template.Must(template.ParseFS(templateFS, "*.go.tmpl"))
}

func render(prefix string, data any) (subject, textBody, htmlBody string, err error) {
	var subjectBuf, textBuf, htmlBuf bytes.Buffer

	if err := tmpls.ExecuteTemplate(&subjectBuf, prefix+"_subject", data); err != nil {
		return "", "", "", fmt.Errorf("render subject: %w", err)
	}
	if err := tmpls.ExecuteTemplate(&textBuf, prefix+"_textBody", data); err != nil {
		return "", "", "", fmt.Errorf("render textBody: %w", err)
	}
	if err := tmpls.ExecuteTemplate(&htmlBuf, prefix+"_htmlBody", data); err != nil {
		return "", "", "", fmt.Errorf("render htmlBody: %w", err)
	}

	return subjectBuf.String(), textBuf.String(), htmlBuf.String(), nil
}

type VerificationData struct {
	Link string
}

func RenderVerificationEmail(link string) (subject, textBody, htmlBody string, err error) {
	return render("verification", VerificationData{Link: link})
}

type PasswordResetData struct {
	Link string
}

func RenderPasswordResetEmail(link string) (subject, textBody, htmlBody string, err error) {
	return render("password_reset", PasswordResetData{Link: link})
}

type WelcomeData struct {
	DashboardURL string
}

func RenderWelcomeEmail(frontendBaseURL string) (subject, textBody, htmlBody string, err error) {
	return render("welcome", WelcomeData{DashboardURL: frontendBaseURL + "/dashboard"})
}

type InvitationData struct {
	InviterEmail   string
	WorkspaceName  string
	Role           string
	InvitationsURL string
}

func RenderInvitationEmail(inviterEmail, workspaceName, role, frontendBaseURL string) (subject, textBody, htmlBody string, err error) {
	return render("invitation", InvitationData{
		InviterEmail:   inviterEmail,
		WorkspaceName:  workspaceName,
		Role:           role,
		InvitationsURL: frontendBaseURL + "/invitations",
	})
}
