/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 18:44:17
 * @FilePath: \electron-go-app\backend\internal\infra\email\aliyun_sender.go
 * @LastEditTime: 2025-10-10 18:44:21
 */
package email

import (
	"context"
	"fmt"
	"strings"

	domain "electron-go-app/backend/internal/domain/user"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dm "github.com/alibabacloud-go/dm-20151123/v2/client"
	"github.com/alibabacloud-go/tea/tea"
)

// AliyunSender 使用阿里云 DirectMail 发送邮箱验证邮件。
type AliyunSender struct {
	client *dm.Client
	cfg    AliyunConfig
}

func NewAliyunSender(cfg AliyunConfig) (*AliyunSender, error) {
	if cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
		return nil, fmt.Errorf("aliyun access key not configured")
	}
	if cfg.AccountName == "" {
		return nil, fmt.Errorf("aliyun account name not configured")
	}

	endpoint := cfg.Endpoint
	if strings.TrimSpace(endpoint) == "" {
		endpoint = "dm.aliyuncs.com"
	}

	openapiCfg := &openapi.Config{
		AccessKeyId:     tea.String(cfg.AccessKeyID),
		AccessKeySecret: tea.String(cfg.AccessKeySecret),
	}
	if cfg.RegionID != "" {
		openapiCfg.RegionId = tea.String(cfg.RegionID)
	}
	openapiCfg.Endpoint = tea.String(endpoint)

	client, err := dm.NewClient(openapiCfg)
	if err != nil {
		return nil, fmt.Errorf("init aliyun directmail client: %w", err)
	}

	return &AliyunSender{client: client, cfg: cfg}, nil
}

func (s *AliyunSender) SendVerification(ctx context.Context, user *domain.User, token string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("aliyun sender not configured")
	}

	subject, textBody, htmlBody := composeVerificationContent(s.cfg.VerificationBaseURL, user, token)

	request := &dm.SingleSendMailRequest{
		AccountName:    tea.String(s.cfg.AccountName),
		ToAddress:      tea.String(user.Email),
		Subject:        tea.String(subject),
		AddressType:    tea.Int32(s.cfg.AddressType),
		ReplyToAddress: tea.Bool(s.cfg.ReplyToAddress),
	}

	if s.cfg.FromAlias != "" {
		request.FromAlias = tea.String(s.cfg.FromAlias)
	}
	if s.cfg.TagName != "" {
		request.TagName = tea.String(s.cfg.TagName)
	}

	if htmlBody != "" {
		request.HtmlBody = tea.String(htmlBody)
	} else {
		request.TextBody = tea.String(textBody)
	}

	// DirectMail SDK 不支持 context 取消，但我们仍在外层等待调用返回。
	_, err := s.client.SingleSendMail(request)
	if err != nil {
		return fmt.Errorf("aliyun single send mail: %w", err)
	}

	return nil
}
