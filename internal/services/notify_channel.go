package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"bss/internal/models"

	"gorm.io/gorm"
)

// 通知渠道相关错误
var (
	ErrChannelUnknown   = errors.New("未知的通知渠道")
	ErrChannelDisabled  = errors.New("该渠道未启用")
	ErrSMTPIncomplete   = errors.New("SMTP 配置不完整（主机/端口/发件人必填）")
	ErrWecomWebhookBad  = errors.New("企业微信 webhook 地址非法")
	ErrNoRecipientEmail = errors.New("收件人邮箱为空")
)

// PasswordMask 读接口返回的密码占位符；PUT 原样回传表示「保持不变」。
const PasswordMask = "********"

// 可替换的外发实现（测试中打桩，避免真实发信）
var (
	notifyHTTPClient = &http.Client{Timeout: 10 * time.Second}
	sendMailFunc     = sendMailSMTP
)

type NotifyService struct {
	db *gorm.DB
}

func NewNotifyService(db *gorm.DB) *NotifyService { return &NotifyService{db: db} }

// ---------- 配置 ----------

// Settings 读取渠道配置；单行缺失时返回「全部关闭」的默认值。
func (s *NotifyService) Settings(ctx context.Context) (*models.NotifySettings, error) {
	var st models.NotifySettings
	err := s.db.WithContext(ctx).Take(&st, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.NotifySettings{ID: 1, SmtpPort: 465, SmtpTLS: true}, nil
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// Masked 返回可安全下发前端的副本（密码只暴露「是否已设置」）。
func Masked(st *models.NotifySettings) map[string]any {
	pwd := ""
	if st.SmtpPassword != "" {
		pwd = PasswordMask
	}
	return map[string]any{
		"email_enabled": st.EmailEnabled,
		"smtp_host":     st.SmtpHost,
		"smtp_port":     st.SmtpPort,
		"smtp_username": st.SmtpUsername,
		"smtp_password": pwd,
		"smtp_from":     st.SmtpFrom,
		"smtp_tls":      st.SmtpTLS,
		"wecom_enabled": st.WecomEnabled,
		"wecom_webhook": st.WecomWebhook,
		"types":         st.Types,
		"updated_at":    st.UpdatedAt,
	}
}

// NotifySettingsInput 可写字段。SmtpPassword 为空或等于掩码时保留原密码。
type NotifySettingsInput struct {
	EmailEnabled bool   `json:"email_enabled"`
	SmtpHost     string `json:"smtp_host"`
	SmtpPort     int    `json:"smtp_port"`
	SmtpUsername string `json:"smtp_username"`
	SmtpPassword string `json:"smtp_password"`
	SmtpFrom     string `json:"smtp_from"`
	SmtpTLS      bool   `json:"smtp_tls"`
	WecomEnabled bool   `json:"wecom_enabled"`
	WecomWebhook string `json:"wecom_webhook"`
	Types        string `json:"types"`
}

func (s *NotifyService) UpdateSettings(ctx context.Context, in NotifySettingsInput) (*models.NotifySettings, error) {
	cur, err := s.Settings(ctx)
	if err != nil {
		return nil, err
	}
	st := models.NotifySettings{
		ID:           1,
		EmailEnabled: in.EmailEnabled,
		SmtpHost:     strings.TrimSpace(in.SmtpHost),
		SmtpPort:     in.SmtpPort,
		SmtpUsername: strings.TrimSpace(in.SmtpUsername),
		SmtpFrom:     strings.TrimSpace(in.SmtpFrom),
		SmtpTLS:      in.SmtpTLS,
		WecomEnabled: in.WecomEnabled,
		WecomWebhook: strings.TrimSpace(in.WecomWebhook),
		Types:        strings.TrimSpace(in.Types),
		UpdatedAt:    time.Now().UTC(),
	}
	// 密码留空或回传掩码 = 沿用原值
	if in.SmtpPassword == "" || in.SmtpPassword == PasswordMask {
		st.SmtpPassword = cur.SmtpPassword
	} else {
		st.SmtpPassword = in.SmtpPassword
	}
	if st.SmtpPort <= 0 {
		st.SmtpPort = 465
	}
	if st.EmailEnabled && (st.SmtpHost == "" || st.SmtpFrom == "") {
		return nil, ErrSMTPIncomplete
	}
	if st.WecomEnabled && !validWebhook(st.WecomWebhook) {
		return nil, ErrWecomWebhookBad
	}
	if err := s.db.WithContext(ctx).Save(&st).Error; err != nil {
		return nil, err
	}
	return &st, nil
}

func validWebhook(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// ---------- 派发 ----------

// Dispatch 把一条站内通知外发到已启用的渠道。任何失败只记日志，不影响站内信。
// 渠道全关时零开销直接返回。
func (s *NotifyService) Dispatch(ctx context.Context, n *models.Notification) {
	st, err := s.Settings(ctx)
	if err != nil || st == nil {
		return // 表缺失/读失败：视为未配置，不打扰主流程
	}
	if !st.EmailEnabled && !st.WecomEnabled {
		return
	}
	if !typeAllowed(st.Types, n.Type) {
		return
	}

	if st.EmailEnabled {
		to := s.emailOf(ctx, n.UserID)
		err := func() error {
			if to == "" {
				return ErrNoRecipientEmail
			}
			return sendMailFunc(st, to, n.Title, plainBody(n))
		}()
		s.writeLog(ctx, models.ChannelEmail, n.ID, n.UserID, to, n.Title, err)
	}
	if st.WecomEnabled {
		err := postWecom(ctx, st.WecomWebhook, markdownBody(n, s.nameOf(ctx, n.UserID)))
		s.writeLog(ctx, models.ChannelWecom, n.ID, n.UserID, hostOf(st.WecomWebhook), n.Title, err)
	}
}

// typeAllowed 空白名单=全部类型放行。
func typeAllowed(list, typ string) bool {
	list = strings.TrimSpace(list)
	if list == "" {
		return true
	}
	for _, t := range strings.Split(list, ",") {
		if strings.TrimSpace(t) == typ {
			return true
		}
	}
	return false
}

// SendTest 向指定渠道发送一条测试消息（不关联通知，日志 notification_id=0）。
func (s *NotifyService) SendTest(ctx context.Context, channel, to string, userID uint) error {
	st, err := s.Settings(ctx)
	if err != nil {
		return err
	}
	title := "【BSS】通知渠道测试"
	body := fmt.Sprintf("这是一条来自 BSS 的测试消息。\n发送时间：%s\n如果你收到它，说明该渠道配置正确。",
		time.Now().Format("2006-01-02 15:04:05"))

	switch channel {
	case models.ChannelEmail:
		if !st.EmailEnabled {
			return ErrChannelDisabled
		}
		if to == "" {
			to = s.emailOf(ctx, userID)
		}
		if to == "" {
			return ErrNoRecipientEmail
		}
		err = sendMailFunc(st, to, title, body)
		s.writeLog(ctx, models.ChannelEmail, 0, userID, to, title, err)
	case models.ChannelWecom:
		if !st.WecomEnabled {
			return ErrChannelDisabled
		}
		err = postWecom(ctx, st.WecomWebhook, "**"+title+"**\n"+body)
		s.writeLog(ctx, models.ChannelWecom, 0, userID, hostOf(st.WecomWebhook), title, err)
	default:
		return ErrChannelUnknown
	}
	return err
}

// Logs 外发日志分页查询（admin 维护用）。
func (s *NotifyService) Logs(ctx context.Context, channel, status string, page, size int) ([]models.NotifyLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	q := s.db.WithContext(ctx).Model(&models.NotifyLog{})
	if channel != "" {
		q = q.Where("channel = ?", channel)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.NotifyLog
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (s *NotifyService) writeLog(ctx context.Context, channel string, nid, uid uint, target, title string, sendErr error) {
	l := models.NotifyLog{
		Channel: channel, NotificationID: nid, UserID: uid,
		Target: target, Title: title, Status: models.NotifySuccess,
		CreatedAt: time.Now().UTC(),
	}
	if sendErr != nil {
		l.Status = models.NotifyFailed
		l.Error = truncate(sendErr.Error(), 500)
	}
	if err := s.db.WithContext(ctx).Create(&l).Error; err != nil {
		log.Printf("[notify] 写外发日志失败: %v", err)
	}
}

func (s *NotifyService) emailOf(ctx context.Context, uid uint) string {
	if uid == 0 {
		return ""
	}
	var e models.Employee
	if err := s.db.WithContext(ctx).First(&e, uid).Error; err != nil {
		return ""
	}
	return e.Email
}

func (s *NotifyService) nameOf(ctx context.Context, uid uint) string {
	if uid == 0 {
		return ""
	}
	var e models.Employee
	if err := s.db.WithContext(ctx).First(&e, uid).Error; err != nil {
		return ""
	}
	return e.Name
}

// ---------- 消息体 ----------

func plainBody(n *models.Notification) string {
	return fmt.Sprintf("%s\n\n%s\n\n—— BSS 业务管理系统自动提醒", n.Title, n.Content)
}

func markdownBody(n *models.Notification, owner string) string {
	who := ""
	if owner != "" {
		who = fmt.Sprintf("\n> 负责人：%s", owner)
	}
	return fmt.Sprintf("**%s**\n> %s%s\n> 时间：%s", n.Title, n.Content, who,
		time.Now().Format("2006-01-02 15:04"))
}

// ---------- 企业微信 ----------

type wecomResp struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// postWecom 发送 markdown 消息到群机器人 webhook。
func postWecom(ctx context.Context, webhook, content string) error {
	if !validWebhook(webhook) {
		return ErrWecomWebhookBad
	}
	payload, _ := json.Marshal(map[string]any{
		"msgtype":  "markdown",
		"markdown": map[string]string{"content": content},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := notifyHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook 返回 HTTP %d", res.StatusCode)
	}
	var wr wecomResp
	if err := json.NewDecoder(res.Body).Decode(&wr); err != nil {
		return fmt.Errorf("webhook 响应解析失败: %w", err)
	}
	if wr.ErrCode != 0 {
		return fmt.Errorf("企业微信返回错误 %d: %s", wr.ErrCode, wr.ErrMsg)
	}
	return nil
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

// ---------- 邮件 ----------

// sendMailSMTP 发送纯文本邮件。SmtpTLS=true 走隐式 TLS（465），否则明文连接后尝试 STARTTLS。
func sendMailSMTP(st *models.NotifySettings, to, subject, body string) error {
	if st.SmtpHost == "" || st.SmtpFrom == "" {
		return ErrSMTPIncomplete
	}
	addr := net.JoinHostPort(st.SmtpHost, fmt.Sprint(st.SmtpPort))
	msg := buildMessage(st.SmtpFrom, to, subject, body)

	var (
		c   *smtp.Client
		err error
	)
	if st.SmtpTLS {
		conn, derr := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr,
			&tls.Config{ServerName: st.SmtpHost, MinVersion: tls.VersionTLS12})
		if derr != nil {
			return derr
		}
		c, err = smtp.NewClient(conn, st.SmtpHost)
		if err != nil {
			conn.Close()
			return err
		}
	} else {
		conn, derr := net.DialTimeout("tcp", addr, 10*time.Second)
		if derr != nil {
			return derr
		}
		c, err = smtp.NewClient(conn, st.SmtpHost)
		if err != nil {
			conn.Close()
			return err
		}
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: st.SmtpHost, MinVersion: tls.VersionTLS12}); err != nil {
				c.Close()
				return err
			}
		}
	}
	defer c.Close()

	if st.SmtpUsername != "" {
		if err := c.Auth(smtp.PlainAuth("", st.SmtpUsername, st.SmtpPassword, st.SmtpHost)); err != nil {
			return err
		}
	}
	if err := c.Mail(st.SmtpFrom); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	wc, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := wc.Write(msg); err != nil {
		wc.Close()
		return err
	}
	if err := wc.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// buildMessage 组装 RFC5322 报文；中文主题按 RFC2047 编码，正文 base64 避免长行截断。
func buildMessage(from, to, subject, body string) []byte {
	var b bytes.Buffer
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	enc := base64.StdEncoding.EncodeToString([]byte(body))
	for len(enc) > 76 {
		b.WriteString(enc[:76] + "\r\n")
		enc = enc[76:]
	}
	b.WriteString(enc + "\r\n")
	return b.Bytes()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
