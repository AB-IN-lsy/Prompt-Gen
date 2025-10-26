/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 18:43:03
 * @FilePath: \electron-go-app\backend\internal\infra\email\template.go
 * @LastEditTime: 2025-10-10 18:43:07
 */
package email

import (
	"fmt"
	"html/template"
	"strings"

	domain "electron-go-app/backend/internal/domain/user"
)

var verificationHTMLTemplate = template.Must(template.New("verification_html").Parse(`<p>Hello {{.Name}},</p>
<p>Welcome to <strong>PromptGen</strong>! Please verify your email by clicking the button below:</p>
<p><a href="{{.URL}}" style="display:inline-block;padding:10px 18px;background:#2563eb;color:#ffffff;text-decoration:none;border-radius:6px;">Verify Email / 邮箱验证</a></p>
<p>Or copy the link: <br><a href="{{.URL}}">{{.URL}}</a></p>
<hr>
<p>您好 {{.Name}}，感谢注册 <strong>PromptGen</strong>。</p>
<p>请点击上方按钮或复制链接完成邮箱验证。</p>
<p>如果这不是您本人的操作，请忽略此邮件。</p>
<p>PromptGen Team</p>`))

// composeVerificationContent 根据用户信息和 token 生成邮件主题与正文。
func composeVerificationContent(baseURL string, user *domain.User, token string) (subject string, textBody string, htmlBody string) {
	verificationURL := buildVerificationURL(baseURL, token)

	subject = "Verify your PromptGen account | 请验证 PromptGen 账户"

	textBody = fmt.Sprintf("Hello %s,\n\nWelcome to PromptGen! Please verify your email by clicking the link below:\n%s\n\nIf you did not create this account, you can ignore this message.\n\n----\n您好 %s，感谢注册 PromptGen。\n请点击下方链接完成邮箱验证：\n%s\n\n如果这不是您本人的操作，请忽略此邮件。\n\nPromptGen Team",
		safeDisplayName(user), verificationURL, safeDisplayName(user), verificationURL,
	)

	tmplData := struct {
		Name string
		URL  string
	}{
		Name: safeDisplayName(user),
		URL:  verificationURL,
	}

	htmlBodyBuilder := new(strings.Builder)
	_ = verificationHTMLTemplate.Execute(htmlBodyBuilder, tmplData)
	htmlBody = htmlBodyBuilder.String()

	return subject, textBody, htmlBody
}

func buildVerificationURL(baseURL, token string) string {
	normalised := normaliseBaseURL(baseURL)
	trimmed := strings.TrimRight(normalised, "/")
	if trimmed == "" {
		return token
	}
	return fmt.Sprintf("%s/email/verified?token=%s", trimmed, token)
}

func safeDisplayName(user *domain.User) string {
	if user == nil {
		return "there"
	}
	name := strings.TrimSpace(user.Username)
	if name != "" {
		return name
	}
	if strings.TrimSpace(user.Email) != "" {
		return strings.TrimSpace(user.Email)
	}
	return "there"
}
