package app

import "fmt"

func WelcomeEmailSubject() string {
	return "Welcome to SendFlow!"
}

func WelcomeEmailTextBody(frontendBaseURL string) string {
	return fmt.Sprintf(`Welcome to SendFlow!

We're excited to have you on board. SendFlow helps you create, send, and track email campaigns with ease.

Get started by visiting your dashboard: %s

If you have any questions, feel free to reach out to our support team.

Best,
The SendFlow Team`, frontendBaseURL+"/dashboard")
}

func WelcomeEmailHTMLBody(frontendBaseURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; padding: 20px;">
<h2>Welcome to SendFlow!</h2>
<p>We're excited to have you on board. SendFlow helps you create, send, and track email campaigns with ease.</p>
<p><a href="%s/dashboard">Visit your dashboard</a> to get started.</p>
<p>If you have any questions, feel free to reach out to our support team.</p>
<p>Best,<br>The SendFlow Team</p>
</body>
</html>`, frontendBaseURL)
}

func InvitationEmailSubject() string {
	return "You've been invited to join a workspace on SendFlow"
}

func InvitationEmailTextBody(inviterEmail, workspaceName, role, frontendBaseURL string) string {
	return fmt.Sprintf(`You've been invited to join a workspace on SendFlow!

%s has invited you to join the workspace "%s" with the role of "%s".

Click the link below to accept the invitation and get started:
%s

If you don't have an account yet, you'll be prompted to create one.

Best,
The SendFlow Team`, inviterEmail, workspaceName, role, frontendBaseURL+"/invitations")
}

func InvitationEmailHTMLBody(inviterEmail, workspaceName, role, frontendBaseURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; padding: 20px;">
<h2>You've been invited to join a workspace!</h2>
<p>%s has invited you to join the workspace <strong>"%s"</strong> with the role of <strong>"%s"</strong>.</p>
<p><a href="%s/invitations">Accept invitation</a> to get started.</p>
<p>If you don't have an account yet, you'll be prompted to create one.</p>
<p>Best,<br>The SendFlow Team</p>
</body>
</html>`, inviterEmail, workspaceName, role, frontendBaseURL)
}
