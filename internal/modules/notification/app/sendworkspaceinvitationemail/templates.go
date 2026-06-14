package sendworkspaceinvitationemail

import "fmt"

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
