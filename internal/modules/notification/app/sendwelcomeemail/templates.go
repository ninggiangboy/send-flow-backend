package sendwelcomeemail

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
