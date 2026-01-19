package service

import (
	"fmt"

	"finance/config"

	"gopkg.in/gomail.v2"
)

// EmailService 邮件服务
type EmailService struct {
	cfg *config.EmailConfig
}

// NewEmailService 创建邮件服务
func NewEmailService(cfg *config.EmailConfig) *EmailService {
	return &EmailService{cfg: cfg}
}

// SendPasswordResetEmail 发送密码重置邮件
func (s *EmailService) SendPasswordResetEmail(toEmail, username, resetLink string) error {
	if !s.cfg.Enabled {
		return fmt.Errorf("邮件服务未启用，请配置 EMAIL_ENABLED=true")
	}

	subject := "【记账系统】密码重置"
	body := s.generateResetEmailBody(username, resetLink)

	return s.sendEmail(toEmail, subject, body)
}

// generateResetEmailBody 生成重置邮件内容
func (s *EmailService) generateResetEmailBody(username, resetLink string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: 'Microsoft YaHei', Arial, sans-serif; background: #f5f5f5; margin: 0; padding: 20px; }
        .container { max-width: 600px; margin: 0 auto; background: #fff; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 20px rgba(0,0,0,0.1); }
        .header { background: linear-gradient(135deg, #2563eb, #1d4ed8); color: white; padding: 30px; text-align: center; }
        .header h1 { margin: 0; font-size: 24px; }
        .content { padding: 40px 30px; }
        .content p { color: #333; line-height: 1.8; margin: 0 0 20px; }
        .btn { display: inline-block; background: linear-gradient(135deg, #2563eb, #1d4ed8); color: white !important; text-decoration: none; padding: 14px 40px; border-radius: 8px; font-weight: 600; margin: 20px 0; }
        .btn:hover { opacity: 0.9; }
        .warning { background: #fff3cd; border-left: 4px solid #ffc107; padding: 15px; margin: 20px 0; border-radius: 4px; }
        .warning p { margin: 0; color: #856404; font-size: 14px; }
        .footer { background: #f8f9fa; padding: 20px 30px; text-align: center; color: #6c757d; font-size: 12px; }
        .link { word-break: break-all; color: #2563eb; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>💰 记账系统</h1>
        </div>
        <div class="content">
            <p>尊敬的 <strong>%s</strong>，您好！</p>
            <p>我们收到了您的密码重置请求。请点击下方按钮重置您的密码：</p>
            <p style="text-align: center;">
                <a href="%s" class="btn">重置密码</a>
            </p>
            <div class="warning">
                <p>⚠️ 此链接有效期为 <strong>30 分钟</strong>，请尽快完成密码重置。</p>
                <p>⚠️ 如果您没有请求重置密码，请忽略此邮件。</p>
            </div>
            <p>如果按钮无法点击，请复制以下链接到浏览器打开：</p>
            <p class="link">%s</p>
        </div>
        <div class="footer">
            <p>此邮件由系统自动发送，请勿回复</p>
            <p>© 记账系统 - 您的个人财务管理助手</p>
        </div>
    </div>
</body>
</html>
`, username, resetLink, resetLink)
}

// sendEmail 发送邮件
func (s *EmailService) sendEmail(to, subject, body string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", m.FormatAddress(s.cfg.Username, s.cfg.From))
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(s.cfg.Host, s.cfg.Port, s.cfg.Username, s.cfg.Password)

	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("发送邮件失败: %w", err)
	}

	return nil
}

// SendTestEmail 发送测试邮件
func (s *EmailService) SendTestEmail(toEmail string) error {
	if !s.cfg.Enabled {
		return fmt.Errorf("邮件服务未启用")
	}

	subject := "【记账系统】邮件配置测试"
	body := `
<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family: Arial, sans-serif; padding: 20px;">
    <h2>✅ 邮件配置成功</h2>
    <p>如果您收到这封邮件，说明邮件服务配置正确。</p>
    <p style="color: #666;">—— 记账系统</p>
</body>
</html>
`
	return s.sendEmail(toEmail, subject, body)
}

// SendVerificationEmail 发送邮箱验证码邮件
func (s *EmailService) SendVerificationEmail(toEmail, code, purpose string) error {
	if !s.cfg.Enabled {
		return fmt.Errorf("邮件服务未启用，请配置 EMAIL_ENABLED=true")
	}

	subject := "【记账系统】邮箱验证码"
	body := s.generateVerificationEmailBody(code, purpose)

	return s.sendEmail(toEmail, subject, body)
}

// generateVerificationEmailBody 生成验证码邮件内容
func (s *EmailService) generateVerificationEmailBody(code, purpose string) string {
	purposeText := "验证您的邮箱"
	if purpose == "register" {
		purposeText = "完成账号注册"
	} else if purpose == "bind" {
		purposeText = "绑定您的邮箱"
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: 'Microsoft YaHei', Arial, sans-serif; background: #f5f5f5; margin: 0; padding: 20px; }
        .container { max-width: 600px; margin: 0 auto; background: #fff; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 20px rgba(0,0,0,0.1); }
        .header { background: linear-gradient(135deg, #10b981, #059669); color: white; padding: 30px; text-align: center; }
        .header h1 { margin: 0; font-size: 24px; }
        .content { padding: 40px 30px; }
        .content p { color: #333; line-height: 1.8; margin: 0 0 20px; }
        .code-box { background: linear-gradient(135deg, #f0fdf4, #dcfce7); border: 2px dashed #10b981; border-radius: 12px; padding: 30px; text-align: center; margin: 30px 0; }
        .code { font-size: 36px; font-weight: bold; color: #059669; letter-spacing: 8px; font-family: 'Courier New', monospace; }
        .warning { background: #fff3cd; border-left: 4px solid #ffc107; padding: 15px; margin: 20px 0; border-radius: 4px; }
        .warning p { margin: 0; color: #856404; font-size: 14px; }
        .footer { background: #f8f9fa; padding: 20px 30px; text-align: center; color: #6c757d; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>💰 记账系统</h1>
        </div>
        <div class="content">
            <p>您好！</p>
            <p>您正在%s，请使用以下验证码：</p>
            <div class="code-box">
                <span class="code">%s</span>
            </div>
            <div class="warning">
                <p>⚠️ 此验证码有效期为 <strong>10 分钟</strong>，请尽快完成验证。</p>
                <p>⚠️ 如果这不是您本人的操作，请忽略此邮件。</p>
            </div>
        </div>
        <div class="footer">
            <p>此邮件由系统自动发送，请勿回复</p>
            <p>© 记账系统 - 您的个人财务管理助手</p>
        </div>
    </div>
</body>
</html>
`, purposeText, code)
}

// SendAppPasswordResetEmail 发送 App 端密码重置验证码邮件
func (s *EmailService) SendAppPasswordResetEmail(toEmail, username, code string) error {
	if !s.cfg.Enabled {
		return fmt.Errorf("邮件服务未启用，请配置 EMAIL_ENABLED=true")
	}

	subject := "【记账系统】密码重置验证码"
	body := s.generateAppResetEmailBody(username, code)

	return s.sendEmail(toEmail, subject, body)
}

// generateAppResetEmailBody 生成 App 端密码重置邮件内容
func (s *EmailService) generateAppResetEmailBody(username, code string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: 'Microsoft YaHei', Arial, sans-serif; background: #f5f5f5; margin: 0; padding: 20px; }
        .container { max-width: 600px; margin: 0 auto; background: #fff; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 20px rgba(0,0,0,0.1); }
        .header { background: linear-gradient(135deg, #2563eb, #1d4ed8); color: white; padding: 30px; text-align: center; }
        .header h1 { margin: 0; font-size: 24px; }
        .content { padding: 40px 30px; }
        .content p { color: #333; line-height: 1.8; margin: 0 0 20px; }
        .code-box { background: linear-gradient(135deg, #eff6ff, #dbeafe); border: 2px dashed #2563eb; border-radius: 12px; padding: 30px; text-align: center; margin: 30px 0; }
        .code { font-size: 36px; font-weight: bold; color: #1d4ed8; letter-spacing: 8px; font-family: 'Courier New', monospace; }
        .warning { background: #fff3cd; border-left: 4px solid #ffc107; padding: 15px; margin: 20px 0; border-radius: 4px; }
        .warning p { margin: 0; color: #856404; font-size: 14px; }
        .footer { background: #f8f9fa; padding: 20px 30px; text-align: center; color: #6c757d; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>💰 记账系统</h1>
        </div>
        <div class="content">
            <p>尊敬的 <strong>%s</strong>，您好！</p>
            <p>我们收到了您的密码重置请求，请使用以下验证码重置您的密码：</p>
            <div class="code-box">
                <span class="code">%s</span>
            </div>
            <div class="warning">
                <p>⚠️ 此验证码有效期为 <strong>10 分钟</strong>，请尽快完成密码重置。</p>
                <p>⚠️ 如果您没有请求重置密码，请忽略此邮件。</p>
            </div>
        </div>
        <div class="footer">
            <p>此邮件由系统自动发送，请勿回复</p>
            <p>© 记账系统 - 您的个人财务管理助手</p>
        </div>
    </div>
</body>
</html>
`, username, code)
}

