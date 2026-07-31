package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bss/internal/models"

	"gorm.io/gorm"
)

// newNotifyDB 建库并写入一个可作为收件人的员工。
func newNotifyDB(t *testing.T) (*gorm.DB, uint) {
	t.Helper()
	db := newTestDB(t)
	e := models.Employee{Name: "张三", Email: "zhangsan@example.com", Role: models.RoleSales, Status: "active"}
	if err := db.Create(&e).Error; err != nil {
		t.Fatalf("建员工失败: %v", err)
	}
	return db, e.ID
}

func TestNotifySettings_DefaultAndMask(t *testing.T) {
	db, _ := newNotifyDB(t)
	svc := NewNotifyService(db)
	ctx := context.Background()

	st, err := svc.Settings(ctx)
	if err != nil {
		t.Fatalf("读配置失败: %v", err)
	}
	if st.EmailEnabled || st.WecomEnabled {
		t.Fatalf("默认应两个渠道均关闭，得到 email=%v wecom=%v", st.EmailEnabled, st.WecomEnabled)
	}

	// 保存带密码的配置
	if _, err := svc.UpdateSettings(ctx, NotifySettingsInput{
		EmailEnabled: true, SmtpHost: "smtp.example.com", SmtpPort: 465,
		SmtpUsername: "u", SmtpPassword: "secret", SmtpFrom: "bss@example.com", SmtpTLS: true,
	}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	st, _ = svc.Settings(ctx)
	if m := Masked(st); m["smtp_password"] != PasswordMask {
		t.Fatalf("密码应以掩码返回，得到 %v", m["smtp_password"])
	}

	// 回传掩码 = 不改密码
	if _, err := svc.UpdateSettings(ctx, NotifySettingsInput{
		EmailEnabled: true, SmtpHost: "smtp.example.com", SmtpPort: 465,
		SmtpUsername: "u", SmtpPassword: PasswordMask, SmtpFrom: "bss@example.com", SmtpTLS: true,
	}); err != nil {
		t.Fatalf("二次保存失败: %v", err)
	}
	st, _ = svc.Settings(ctx)
	if st.SmtpPassword != "secret" {
		t.Fatalf("回传掩码时应保留原密码，得到 %q", st.SmtpPassword)
	}
}

func TestNotifySettings_Validation(t *testing.T) {
	db, _ := newNotifyDB(t)
	svc := NewNotifyService(db)
	ctx := context.Background()

	if _, err := svc.UpdateSettings(ctx, NotifySettingsInput{EmailEnabled: true, SmtpHost: ""}); !errors.Is(err, ErrSMTPIncomplete) {
		t.Fatalf("启用邮件但缺主机应报错，得到 %v", err)
	}
	if _, err := svc.UpdateSettings(ctx, NotifySettingsInput{WecomEnabled: true, WecomWebhook: "not-a-url"}); !errors.Is(err, ErrWecomWebhookBad) {
		t.Fatalf("非法 webhook 应报错，得到 %v", err)
	}
}

func TestDispatch_DisabledIsNoop(t *testing.T) {
	db, uid := newNotifyDB(t)
	svc := NewNotifyService(db)
	n := models.Notification{ID: 1, UserID: uid, Type: NotifContractExpiring, Title: "T", Content: "C"}
	svc.Dispatch(context.Background(), &n)

	var cnt int64
	db.Model(&models.NotifyLog{}).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("渠道关闭时不应产生外发日志，得到 %d 条", cnt)
	}
}

func TestDispatch_WecomSuccessAndLog(t *testing.T) {
	db, uid := newNotifyDB(t)
	svc := NewNotifyService(db)
	ctx := context.Background()

	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer srv.Close()

	if _, err := svc.UpdateSettings(ctx, NotifySettingsInput{
		WecomEnabled: true, WecomWebhook: srv.URL + "/cgi-bin/webhook/send?key=x",
	}); err != nil {
		t.Fatalf("保存 webhook 失败: %v", err)
	}

	n := models.Notification{ID: 7, UserID: uid, Type: NotifPaymentOverdue, Title: "回款逾期", Content: "第 1 期已逾期"}
	svc.Dispatch(ctx, &n)

	if got["msgtype"] != "markdown" {
		t.Fatalf("应发送 markdown 消息，得到 %v", got["msgtype"])
	}
	md, _ := got["markdown"].(map[string]any)
	if c, _ := md["content"].(string); !strings.Contains(c, "回款逾期") || !strings.Contains(c, "张三") {
		t.Fatalf("消息内容缺少标题或负责人：%q", c)
	}

	var logs []models.NotifyLog
	db.Find(&logs)
	if len(logs) != 1 || logs[0].Channel != models.ChannelWecom || logs[0].Status != models.NotifySuccess {
		t.Fatalf("应有 1 条成功的 wecom 日志，得到 %+v", logs)
	}
	if logs[0].NotificationID != 7 {
		t.Fatalf("日志应关联通知 7，得到 %d", logs[0].NotificationID)
	}
}

func TestDispatch_WecomErrCodeRecordedAsFailed(t *testing.T) {
	db, uid := newNotifyDB(t)
	svc := NewNotifyService(db)
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":93000,"errmsg":"invalid webhook url"}`))
	}))
	defer srv.Close()

	_, _ = svc.UpdateSettings(ctx, NotifySettingsInput{WecomEnabled: true, WecomWebhook: srv.URL})
	n := models.Notification{ID: 8, UserID: uid, Type: NotifContractExpiring, Title: "合同到期", Content: "x"}
	svc.Dispatch(ctx, &n)

	var l models.NotifyLog
	if err := db.Order("id DESC").First(&l).Error; err != nil {
		t.Fatalf("读日志失败: %v", err)
	}
	if l.Status != models.NotifyFailed || !strings.Contains(l.Error, "93000") {
		t.Fatalf("应记录失败与错误码，得到 %+v", l)
	}
}

func TestDispatch_TypeWhitelist(t *testing.T) {
	db, uid := newNotifyDB(t)
	svc := NewNotifyService(db)
	ctx := context.Background()

	hit := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit++
		_, _ = w.Write([]byte(`{"errcode":0}`))
	}))
	defer srv.Close()

	_, _ = svc.UpdateSettings(ctx, NotifySettingsInput{
		WecomEnabled: true, WecomWebhook: srv.URL, Types: NotifPaymentOverdue,
	})
	// 白名单外的类型：跳过
	svc.Dispatch(ctx, &models.Notification{ID: 1, UserID: uid, Type: NotifContractExpiring, Title: "a"})
	if hit != 0 {
		t.Fatalf("白名单外类型不应外发，命中 %d 次", hit)
	}
	// 白名单内：发送
	svc.Dispatch(ctx, &models.Notification{ID: 2, UserID: uid, Type: NotifPaymentOverdue, Title: "b"})
	if hit != 1 {
		t.Fatalf("白名单内类型应外发 1 次，命中 %d 次", hit)
	}
}

func TestSendTest_EmailUsesStubAndLogs(t *testing.T) {
	db, uid := newNotifyDB(t)
	svc := NewNotifyService(db)
	ctx := context.Background()

	orig := sendMailFunc
	defer func() { sendMailFunc = orig }()
	var toGot, subjGot string
	sendMailFunc = func(st *models.NotifySettings, to, subject, body string) error {
		toGot, subjGot = to, subject
		return nil
	}

	_, _ = svc.UpdateSettings(ctx, NotifySettingsInput{
		EmailEnabled: true, SmtpHost: "smtp.example.com", SmtpPort: 465,
		SmtpFrom: "bss@example.com", SmtpTLS: true,
	})
	if err := svc.SendTest(ctx, models.ChannelEmail, "", uid); err != nil {
		t.Fatalf("测试发送失败: %v", err)
	}
	if toGot != "zhangsan@example.com" {
		t.Fatalf("未指定收件人时应发给当前用户，得到 %q", toGot)
	}
	if !strings.Contains(subjGot, "测试") {
		t.Fatalf("测试邮件主题异常: %q", subjGot)
	}

	list, total, err := svc.Logs(ctx, models.ChannelEmail, "", 1, 10)
	if err != nil || total != 1 || list[0].NotificationID != 0 {
		t.Fatalf("应有 1 条 notification_id=0 的测试日志，total=%d err=%v", total, err)
	}
}

func TestSendTest_ChannelDisabled(t *testing.T) {
	db, uid := newNotifyDB(t)
	svc := NewNotifyService(db)
	if err := svc.SendTest(context.Background(), models.ChannelWecom, "", uid); !errors.Is(err, ErrChannelDisabled) {
		t.Fatalf("未启用渠道应报错，得到 %v", err)
	}
	if err := svc.SendTest(context.Background(), "sms", "", uid); !errors.Is(err, ErrChannelUnknown) {
		t.Fatalf("未知渠道应报错，得到 %v", err)
	}
}

func TestBuildMessage_SubjectEncoded(t *testing.T) {
	msg := string(buildMessage("a@b.com", "c@d.com", "合同到期提醒", "正文"))
	if !strings.Contains(msg, "Subject: =?UTF-8?") {
		t.Fatalf("中文主题应做 RFC2047 编码：%s", msg)
	}
	if !strings.Contains(msg, "Content-Transfer-Encoding: base64") {
		t.Fatalf("正文应 base64 编码：%s", msg)
	}
}
