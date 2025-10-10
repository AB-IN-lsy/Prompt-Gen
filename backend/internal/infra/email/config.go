/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 17:00:12
 * @FilePath: \electron-go-app\backend\internal\infra\email\config.go
 * @LastEditTime: 2025-10-10 17:00:16
 */
package email

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// SMTPConfig 描述 SMTP 邮件发送所需的环境配置。
type SMTPConfig struct {
	Host                string
	Port                int
	Username            string
	Password            string
	From                string
	VerificationBaseURL string
}

// AliyunConfig 描述阿里云邮件推送（DirectMail）的必要配置。
type AliyunConfig struct {
	AccessKeyID         string
	AccessKeySecret     string
	RegionID            string
	AccountName         string
	FromAlias           string
	TagName             string
	ReplyToAddress      bool
	Endpoint            string
	AddressType         int32
	VerificationBaseURL string
}

// LoadSMTPConfigFromEnv 从环境变量读取 SMTP 配置。
// 返回值：配置、是否启用、错误。
func LoadSMTPConfigFromEnv() (SMTPConfig, bool, error) {
	host := os.Getenv("SMTP_HOST")
	portStr := os.Getenv("SMTP_PORT")
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	from := os.Getenv("SMTP_FROM")

	if host == "" || portStr == "" || from == "" {
		return SMTPConfig{}, false, nil
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return SMTPConfig{}, false, fmt.Errorf("parse SMTP_PORT: %w", err)
	}

	baseURL := os.Getenv("APP_PUBLIC_BASE_URL")

	return SMTPConfig{
		Host:                host,
		Port:                port,
		Username:            username,
		Password:            password,
		From:                from,
		VerificationBaseURL: baseURL,
	}, true, nil
}

// LoadAliyunConfigFromEnv 从环境变量读取阿里云邮件推送配置。
// 返回值：配置、是否启用、错误。
func LoadAliyunConfigFromEnv() (AliyunConfig, bool, error) {
	accessKey := strings.TrimSpace(os.Getenv("ALIYUN_DM_ACCESS_KEY_ID"))
	secret := strings.TrimSpace(os.Getenv("ALIYUN_DM_ACCESS_KEY_SECRET"))
	region := strings.TrimSpace(os.Getenv("ALIYUN_DM_REGION_ID"))
	accountName := strings.TrimSpace(os.Getenv("ALIYUN_DM_ACCOUNT_NAME"))

	if accessKey == "" || secret == "" || region == "" || accountName == "" {
		return AliyunConfig{}, false, nil
	}

	fromAlias := strings.TrimSpace(os.Getenv("ALIYUN_DM_FROM_ALIAS"))
	tagName := strings.TrimSpace(os.Getenv("ALIYUN_DM_TAG_NAME"))
	endpoint := strings.TrimSpace(os.Getenv("ALIYUN_DM_ENDPOINT"))
	if endpoint == "" {
		endpoint = "dm.aliyuncs.com"
	}

	replyStr := strings.TrimSpace(os.Getenv("ALIYUN_DM_REPLY_TO_ADDRESS"))
	replyToAddress := true
	if replyStr != "" {
		parsed, err := strconv.ParseBool(replyStr)
		if err != nil {
			return AliyunConfig{}, false, fmt.Errorf("parse ALIYUN_DM_REPLY_TO_ADDRESS: %w", err)
		}
		replyToAddress = parsed
	}

	addressTypeStr := strings.TrimSpace(os.Getenv("ALIYUN_DM_ADDRESS_TYPE"))
	addressType := int32(1)
	if addressTypeStr != "" {
		parsed, err := strconv.Atoi(addressTypeStr)
		if err != nil {
			return AliyunConfig{}, false, fmt.Errorf("parse ALIYUN_DM_ADDRESS_TYPE: %w", err)
		}
		if parsed == 1 {
			addressType = 1
		} else if parsed == 0 {
			// AddressType=0 会由 DirectMail 生成随机发件地址（spam alias），这里强制退回到 1。
			addressType = 1
		} else {
			return AliyunConfig{}, false, fmt.Errorf("invalid ALIYUN_DM_ADDRESS_TYPE: %d", parsed)
		}
	}

	baseURL := os.Getenv("APP_PUBLIC_BASE_URL")

	return AliyunConfig{
		AccessKeyID:         accessKey,
		AccessKeySecret:     secret,
		RegionID:            region,
		AccountName:         accountName,
		FromAlias:           fromAlias,
		TagName:             tagName,
		ReplyToAddress:      replyToAddress,
		Endpoint:            endpoint,
		AddressType:         addressType,
		VerificationBaseURL: baseURL,
	}, true, nil
}
