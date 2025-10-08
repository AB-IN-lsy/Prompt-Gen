/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 17:16:33
 * @FilePath: \electron-go-app\backend\internal\infra\nacos_client.go
 * @LastEditTime: 2025-10-08 19:28:59
 */
package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"electron-go-app/backend/internal/config"
)

const (
	envNacosEndpoint  = "NACOS_ENDPOINT"
	envNacosNamespace = "NACOS_NAMESPACE"
	envNacosUsername  = "NACOS_USERNAME"
	envNacosPassword  = "NACOS_PASSWORD"
	envNacosGroup     = "NACOS_GROUP"
	envNacosContext   = "NACOS_CONTEXT_PATH"
	envNacosScheme    = "NACOS_SCHEME"
)

// Default fallback values that are safe to commit. Real environments must override via env vars.
var (
	defaultScheme    = "http"
	defaultContext   = "/nacos"
	defaultGroup     = "DEFAULT_GROUP"
	defaultNamespace = "public"
)

// NacosOptions 描述访问 Nacos 配置中心所需的连接参数。
type NacosOptions struct {
	Scheme      string
	Host        string
	Port        uint64
	ContextPath string
	NamespaceID string
	Group       string
	Username    string
	Password    string
	Timeout     time.Duration
}

// NewDefaultNacosOptions 从环境变量构建 Nacos 连接配置，未设置时采用默认值。
func NewDefaultNacosOptions() (NacosOptions, error) {
	config.LoadEnvFiles()

	endpoint := strings.TrimSpace(os.Getenv(envNacosEndpoint))
	if endpoint == "" {
		return NacosOptions{}, fmt.Errorf("%s not set", envNacosEndpoint)
	}

	host, port, err := splitHostPort(endpoint)
	if err != nil {
		return NacosOptions{}, fmt.Errorf("nacos endpoint invalid: %w", err)
	}

	scheme := strings.TrimSpace(os.Getenv(envNacosScheme))
	if scheme == "" {
		scheme = defaultScheme
	}

	username := os.Getenv(envNacosUsername)
	password := os.Getenv(envNacosPassword)
	if username == "" {
		return NacosOptions{}, fmt.Errorf("%s not set", envNacosUsername)
	}
	if password == "" {
		return NacosOptions{}, fmt.Errorf("%s not set", envNacosPassword)
	}

	group := os.Getenv(envNacosGroup)
	if group == "" {
		group = defaultGroup
	}

	namespace := os.Getenv(envNacosNamespace)
	if namespace == "" {
		namespace = defaultNamespace
	}

	contextPath := strings.TrimSpace(os.Getenv(envNacosContext))
	if contextPath == "" {
		contextPath = defaultContext
	}
	contextPath = normalizeContextPath(contextPath)

	return NacosOptions{
		Scheme:      scheme,
		Host:        host,
		Port:        port,
		ContextPath: contextPath,
		NamespaceID: namespace,
		Group:       group,
		Username:    username,
		Password:    password,
		Timeout:     15 * time.Second,
	}, nil
}

// FetchNacosConfig 登录 Nacos 并通过 HTTP API 获取配置文本。
func FetchNacosConfig(ctx context.Context, opts NacosOptions, dataID, group string) (string, error) {
	if strings.TrimSpace(dataID) == "" {
		return "", fmt.Errorf("nacos dataID cannot be empty")
	}

	if group == "" {
		group = defaultGroup
	}

	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}

	client := &http.Client{Timeout: opts.Timeout}

	token, err := nacosLogin(ctx, client, opts)
	if err != nil {
		return "", err
	}

	configURL, err := buildConfigURL(opts, dataID, group)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, configURL, nil)
	if err != nil {
		return "", fmt.Errorf("build nacos config request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request nacos config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("nacos config request failed: %s - %s", resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read nacos config body: %w", err)
	}

	return string(body), nil
}

// nacosLogin 调用登录接口并返回 accessToken。
func nacosLogin(ctx context.Context, client *http.Client, opts NacosOptions) (string, error) {
	loginURL := buildBaseURL(opts) + "/v1/auth/login"
	form := url.Values{}
	form.Set("username", opts.Username)
	form.Set("password", opts.Password)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build nacos login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request nacos login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("nacos login failed: %s - %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload struct {
		AccessToken string `json:"accessToken"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode nacos login response: %w", err)
	}

	if payload.AccessToken == "" {
		return "", fmt.Errorf("nacos login returned empty accessToken")
	}

	return payload.AccessToken, nil
}

// buildConfigURL 生成读取配置的完整 URL。
func buildConfigURL(opts NacosOptions, dataID, group string) (string, error) {
	base := buildBaseURL(opts) + "/v1/cs/configs"
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse nacos config url: %w", err)
	}

	query := u.Query()
	query.Set("dataId", dataID)
	query.Set("group", group)
	if opts.NamespaceID != "" {
		query.Set("tenant", opts.NamespaceID)
	}
	u.RawQuery = query.Encode()

	return u.String(), nil
}

// buildBaseURL 拼装请求的基础地址。
func buildBaseURL(opts NacosOptions) string {
	scheme := opts.Scheme
	if scheme == "" {
		scheme = defaultScheme
	}

	portSegment := ""
	if opts.Port != 0 {
		portSegment = fmt.Sprintf(":%d", opts.Port)
	}

	return fmt.Sprintf("%s://%s%s%s", scheme, opts.Host, portSegment, opts.ContextPath)
}

// normalizeContextPath 规范化 context path，确保格式一致。
func normalizeContextPath(path string) string {
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(path, "/")
}

// splitHostPort 拆分 endpoint 字符串，缺省端口时返回 8848。
func splitHostPort(endpoint string) (string, uint64, error) {
	if !strings.Contains(endpoint, ":") {
		return endpoint, 8848, nil
	}

	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", 0, err
	}

	port, err := strconv.ParseUint(portStr, 10, 64)
	if err != nil {
		return "", 0, err
	}

	return host, port, nil
}
