package promptinfra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	promptdomain "electron-go-app/backend/internal/domain/prompt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	defaultWorkspaceTTL          = 45 * time.Minute
	defaultWorkspacePrefix       = "prompt:workspace"
	workspaceFieldTopic          = "topic"
	workspaceFieldLanguage       = "language"
	workspaceFieldModelKey       = "model_key"
	workspaceFieldDraftBody      = "draft_body"
	workspaceFieldVersion        = "version"
	workspaceFieldUpdatedAt      = "updated_at"
	workspaceFieldPromptID       = "prompt_id"
	workspaceFieldStatus         = "status"
	workspaceFieldAttributesJSON = "attributes"
)

// WorkspaceStore 提供 Prompt 工作区的 Redis 存取能力。
type WorkspaceStore struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

// WorkspaceOption 自定义 WorkspaceStore 的行为。
type WorkspaceOption func(*WorkspaceStore)

// WithWorkspacePrefix 覆盖默认的 key 前缀。
func WithWorkspacePrefix(prefix string) WorkspaceOption {
	return func(store *WorkspaceStore) {
		store.prefix = strings.TrimSuffix(prefix, ":")
	}
}

// WithWorkspaceTTL 设置自定义 TTL。
func WithWorkspaceTTL(ttl time.Duration) WorkspaceOption {
	return func(store *WorkspaceStore) {
		if ttl > 0 {
			store.ttl = ttl
		}
	}
}

// NewWorkspaceStore 构造 WorkspaceStore。
func NewWorkspaceStore(client *redis.Client, opts ...WorkspaceOption) *WorkspaceStore {
	store := &WorkspaceStore{
		client: client,
		prefix: defaultWorkspacePrefix,
		ttl:    defaultWorkspaceTTL,
	}
	for _, opt := range opts {
		opt(store)
	}
	return store
}

// CreateOrReplace 创建新的工作区或覆盖已有工作区，返回最终的 workspace token。
func (s *WorkspaceStore) CreateOrReplace(ctx context.Context, userID uint, snapshot promptdomain.WorkspaceSnapshot) (string, error) {
	if s == nil || s.client == nil {
		return "", fmt.Errorf("workspace store not initialised")
	}
	token := strings.TrimSpace(snapshot.Token)
	if token == "" {
		token = uuid.NewString()
	}
	baseKey := s.baseKey(userID, token)
	now := time.Now()
	if snapshot.UpdatedAt.IsZero() {
		snapshot.UpdatedAt = now
	}
	payload := map[string]any{
		workspaceFieldTopic:     strings.TrimSpace(snapshot.Topic),
		workspaceFieldLanguage:  strings.TrimSpace(snapshot.Language),
		workspaceFieldModelKey:  strings.TrimSpace(snapshot.ModelKey),
		workspaceFieldDraftBody: snapshot.DraftBody,
		workspaceFieldVersion:   snapshot.Version,
		workspaceFieldUpdatedAt: snapshot.UpdatedAt.Unix(),
	}
	payload[workspaceFieldPromptID] = snapshot.PromptID
	if strings.TrimSpace(snapshot.Status) != "" {
		payload[workspaceFieldStatus] = strings.TrimSpace(snapshot.Status)
	}
	if len(snapshot.Attributes) > 0 {
		rawAttr, err := json.Marshal(snapshot.Attributes)
		if err != nil {
			return "", fmt.Errorf("encode workspace attributes: %w", err)
		}
		payload[workspaceFieldAttributesJSON] = string(rawAttr)
	}

	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, baseKey, payload)
	if err := s.replaceKeywords(ctx, pipe, baseKey, snapshot.Positive, true); err != nil {
		pipe.Discard()
		return "", err
	}
	if err := s.replaceKeywords(ctx, pipe, baseKey, snapshot.Negative, false); err != nil {
		pipe.Discard()
		return "", err
	}
	s.applyTTL(ctx, pipe, baseKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("store workspace snapshot: %w", err)
	}
	return token, nil
}

// MergeKeywords 将关键词合并到现有工作区中（存在则覆盖来源/权重信息）。
func (s *WorkspaceStore) MergeKeywords(ctx context.Context, userID uint, token string, keywords []promptdomain.WorkspaceKeyword) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("workspace store not initialised")
	}
	if len(keywords) == 0 {
		return nil
	}
	baseKey := s.baseKey(userID, token)
	pipe := s.client.TxPipeline()
	if err := s.mergeKeywords(ctx, pipe, baseKey, keywords); err != nil {
		pipe.Discard()
		return err
	}
	touchUpdatedAt(ctx, pipe, baseKey)
	s.applyTTL(ctx, pipe, baseKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("merge keywords: %w", err)
	}
	return nil
}

// RemoveKeyword 将指定关键词从工作区移除，保持前端与 Redis 数据同步。
func (s *WorkspaceStore) RemoveKeyword(ctx context.Context, userID uint, token, polarity, word string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("workspace store not initialised")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("workspace token is empty")
	}
	lowered := strings.ToLower(strings.TrimSpace(word))
	if lowered == "" {
		return nil
	}
	pol := normalizePolarity(polarity)
	baseKey := s.baseKey(userID, token)
	field := fmt.Sprintf("%s|%s", pol, lowered)
	targetZSet := s.keyPositive(baseKey)
	if pol == promptdomain.KeywordPolarityNegative {
		targetZSet = s.keyNegative(baseKey)
	}

	pipe := s.client.TxPipeline()
	pipe.HDel(ctx, s.keyKeywords(baseKey), field)
	pipe.ZRem(ctx, targetZSet, lowered)
	touchUpdatedAt(ctx, pipe, baseKey)
	s.applyTTL(ctx, pipe, baseKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("remove workspace keyword: %w", err)
	}
	return nil
}

// UpdateDraftBody 更新草稿正文，并刷新更新时间。
func (s *WorkspaceStore) UpdateDraftBody(ctx context.Context, userID uint, token, body string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("workspace store not initialised")
	}
	baseKey := s.baseKey(userID, token)
	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, baseKey, map[string]any{
		workspaceFieldDraftBody: body,
		workspaceFieldUpdatedAt: time.Now().Unix(),
	})
	s.applyTTL(ctx, pipe, baseKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("update workspace draft: %w", err)
	}
	return nil
}

// Touch 刷新工作区的 TTL。
// 每次有新操作都刷新 TTL，确保整段编辑流程都能在缓存里完成
func (s *WorkspaceStore) Touch(ctx context.Context, userID uint, token string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("workspace store not initialised")
	}
	baseKey := s.baseKey(userID, token)
	if err := s.touch(ctx, baseKey); err != nil {
		return err
	}
	return nil
}

// Snapshot 读取完整工作区数据。
func (s *WorkspaceStore) Snapshot(ctx context.Context, userID uint, token string) (promptdomain.WorkspaceSnapshot, error) {
	if s == nil || s.client == nil {
		return promptdomain.WorkspaceSnapshot{}, fmt.Errorf("workspace store not initialised")
	}
	baseKey := s.baseKey(userID, token)
	values, err := s.client.HGetAll(ctx, baseKey).Result()
	if err != nil {
		return promptdomain.WorkspaceSnapshot{}, fmt.Errorf("load workspace hash: %w", err)
	}
	if len(values) == 0 {
		return promptdomain.WorkspaceSnapshot{}, redis.Nil
	}

	snapshot := promptdomain.WorkspaceSnapshot{
		Token:    token,
		UserID:   userID,
		Topic:    values[workspaceFieldTopic],
		Language: values[workspaceFieldLanguage],
		ModelKey: values[workspaceFieldModelKey],
		DraftBody: func() string {
			if v, ok := values[workspaceFieldDraftBody]; ok {
				return v
			}
			return ""
		}(),
		Status: values[workspaceFieldStatus],
	}
	if idStr, ok := values[workspaceFieldPromptID]; ok && strings.TrimSpace(idStr) != "" {
		if parsed, convErr := parseUint(idStr); convErr == nil {
			snapshot.PromptID = parsed
		}
	}
	if ts, ok := values[workspaceFieldUpdatedAt]; ok && ts != "" {
		if unix, convErr := parseInt64(ts); convErr == nil {
			snapshot.UpdatedAt = time.Unix(unix, 0)
		}
	}
	if verStr, ok := values[workspaceFieldVersion]; ok && verStr != "" {
		if version, convErr := parseInt64(verStr); convErr == nil {
			snapshot.Version = version
		}
	}
	if attrRaw, ok := values[workspaceFieldAttributesJSON]; ok && attrRaw != "" {
		var attrs map[string]string
		if decodeErr := json.Unmarshal([]byte(attrRaw), &attrs); decodeErr == nil {
			snapshot.Attributes = attrs
		}
	}

	positive, err := s.readKeywords(ctx, baseKey, true)
	if err != nil {
		return promptdomain.WorkspaceSnapshot{}, err
	}
	negative, err := s.readKeywords(ctx, baseKey, false)
	if err != nil {
		return promptdomain.WorkspaceSnapshot{}, err
	}
	snapshot.Positive = positive
	snapshot.Negative = negative
	return snapshot, nil
}

// Delete 移除整个工作区。
func (s *WorkspaceStore) Delete(ctx context.Context, userID uint, token string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("workspace store not initialised")
	}
	baseKey := s.baseKey(userID, token)
	keys := []string{
		baseKey,
		s.keyPositive(baseKey),
		s.keyNegative(baseKey),
		s.keyKeywords(baseKey),
	}
	if err := s.client.Del(ctx, keys...).Err(); err != nil && err != redis.Nil {
		return fmt.Errorf("delete workspace: %w", err)
	}
	return nil
}

// SetPromptMeta 将 prompt_id/status 等元信息写入工作区。
func (s *WorkspaceStore) SetPromptMeta(ctx context.Context, userID uint, token string, promptID uint, status string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("workspace store not initialised")
	}
	baseKey := s.baseKey(userID, token)
	values := map[string]any{
		workspaceFieldPromptID:  promptID,
		workspaceFieldStatus:    strings.TrimSpace(status),
		workspaceFieldUpdatedAt: time.Now().Unix(),
	}
	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, baseKey, values)
	s.applyTTL(ctx, pipe, baseKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("set prompt meta: %w", err)
	}
	return nil
}

// GetPromptMeta 读取工作区内缓存的 prompt_id 与状态信息。
func (s *WorkspaceStore) GetPromptMeta(ctx context.Context, userID uint, token string) (uint, string, error) {
	if s == nil || s.client == nil {
		return 0, "", fmt.Errorf("workspace store not initialised")
	}
	baseKey := s.baseKey(userID, token)
	values, err := s.client.HMGet(ctx, baseKey, workspaceFieldPromptID, workspaceFieldStatus).Result()
	if err != nil {
		return 0, "", fmt.Errorf("get prompt meta: %w", err)
	}
	if len(values) == 0 {
		return 0, "", redis.Nil
	}
	var promptID uint
	if raw := values[0]; raw != nil {
		if str, ok := raw.(string); ok && strings.TrimSpace(str) != "" {
			if parsed, convErr := parseUint(str); convErr == nil {
				promptID = parsed
			}
		}
	}
	status := ""
	if len(values) > 1 {
		if str, ok := values[1].(string); ok {
			status = str
		}
	}
	return promptID, status, nil
}

// replaceKeywords 使用提供的关键词集合重建指定极性的 ZSET。
func (s *WorkspaceStore) replaceKeywords(ctx context.Context, pipe redis.Pipeliner, baseKey string, keywords []promptdomain.WorkspaceKeyword, positive bool) error {
	zsetKey := s.keyPositive(baseKey)
	if !positive {
		zsetKey = s.keyNegative(baseKey)
	}
	pipe.Del(ctx, zsetKey)
	for _, kw := range keywords {
		if err := s.enqueueKeyword(ctx, pipe, baseKey, kw); err != nil {
			return err
		}
	}
	return nil
}

// mergeKeywords 将关键词追加到现有集合中，存在则覆盖原有属性。
func (s *WorkspaceStore) mergeKeywords(ctx context.Context, pipe redis.Pipeliner, baseKey string, keywords []promptdomain.WorkspaceKeyword) error {
	for _, kw := range keywords {
		if err := s.enqueueKeyword(ctx, pipe, baseKey, kw); err != nil {
			return err
		}
	}
	return nil
}

// enqueueKeyword 将单个关键词写入 Hash 与 ZSET，维持排序信息。
func (s *WorkspaceStore) enqueueKeyword(ctx context.Context, pipe redis.Pipeliner, baseKey string, keyword promptdomain.WorkspaceKeyword) error {
	if keyword.Word == "" {
		return nil
	}
	lowered := strings.ToLower(keyword.Word)
	field := fmt.Sprintf("%s|%s", normalizePolarity(keyword.Polarity), lowered)
	payload, err := json.Marshal(keyword)
	if err != nil {
		return fmt.Errorf("encode keyword: %w", err)
	}
	pipe.HSet(ctx, s.keyKeywords(baseKey), field, payload)
	score := keyword.Score
	if score == 0 {
		score = float64(time.Now().UnixNano())
	}
	targetZSet := s.keyPositive(baseKey)
	if normalizePolarity(keyword.Polarity) == promptdomain.KeywordPolarityNegative {
		targetZSet = s.keyNegative(baseKey)
	}
	pipe.ZAdd(ctx, targetZSet, redis.Z{
		Score:  score,
		Member: lowered,
	})
	return nil
}

// readKeywords 读取指定极性的关键词集合，并恢复 payload 信息。
func (s *WorkspaceStore) readKeywords(ctx context.Context, baseKey string, positive bool) ([]promptdomain.WorkspaceKeyword, error) {
	zsetKey := s.keyPositive(baseKey)
	if !positive {
		zsetKey = s.keyNegative(baseKey)
	}
	members, err := s.client.ZRangeWithScores(ctx, zsetKey, 0, -1).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("read workspace keywords: %w", err)
	}
	if len(members) == 0 {
		return nil, nil
	}
	fields := make([]string, 0, len(members))
	for _, member := range members {
		word := fmt.Sprintf("%v", member.Member)
		polarity := promptdomain.KeywordPolarityPositive
		if !positive {
			polarity = promptdomain.KeywordPolarityNegative
		}
		fields = append(fields, fmt.Sprintf("%s|%s", polarity, word))
	}
	values, err := s.client.HMGet(ctx, s.keyKeywords(baseKey), fields...).Result()
	if err != nil {
		return nil, fmt.Errorf("read keyword payloads: %w", err)
	}
	result := make([]promptdomain.WorkspaceKeyword, 0, len(fields))
	for idx, raw := range values {
		if raw == nil {
			continue
		}
		payload, ok := raw.(string)
		if !ok {
			continue
		}
		var entity promptdomain.WorkspaceKeyword
		if err := json.Unmarshal([]byte(payload), &entity); err != nil {
			continue
		}
		entity.Score = members[idx].Score
		result = append(result, entity)
	}
	return result, nil
}

// touch 刷新工作区所有相关 key 的 TTL。
func (s *WorkspaceStore) touch(ctx context.Context, baseKey string) error {
	pipe := s.client.TxPipeline()
	s.applyTTL(ctx, pipe, baseKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("touch workspace ttl: %w", err)
	}
	return nil
}

// applyTTL 批量更新工作区核心 key 的过期时间。
func (s *WorkspaceStore) applyTTL(ctx context.Context, pipe redis.Pipeliner, baseKey string) {
	keys := []string{
		baseKey,
		s.keyPositive(baseKey),
		s.keyNegative(baseKey),
		s.keyKeywords(baseKey),
	}
	for _, key := range keys {
		pipe.Expire(ctx, key, s.ttl)
	}
}

// baseKey 生成工作区主 Hash 的 Redis key。
func (s *WorkspaceStore) baseKey(userID uint, token string) string {
	return fmt.Sprintf("%s:%d:%s", s.prefix, userID, token)
}

// keyPositive 返回正向关键词 ZSET 的 key。
func (s *WorkspaceStore) keyPositive(baseKey string) string {
	return baseKey + ":positive"
}

// keyNegative 返回负向关键词 ZSET 的 key。
func (s *WorkspaceStore) keyNegative(baseKey string) string {
	return baseKey + ":negative"
}

// keyKeywords 返回关键词明细 Hash 的 key。
func (s *WorkspaceStore) keyKeywords(baseKey string) string {
	return baseKey + ":keywords"
}

// normalizePolarity 规范化极性字段，缺省时视为正向。
func normalizePolarity(p string) string {
	if strings.EqualFold(strings.TrimSpace(p), promptdomain.KeywordPolarityNegative) {
		return promptdomain.KeywordPolarityNegative
	}
	return promptdomain.KeywordPolarityPositive
}

// parseInt64 辅助解析字符串为 int64。
func parseInt64(val string) (int64, error) {
	return strconv.ParseInt(val, 10, 64)
}

// parseUint 辅助解析字符串为 uint。
func parseUint(val string) (uint, error) {
	parsed, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(parsed), nil
}

// touchUpdatedAt 更新工作区 Hash 的最近修改时间。
func touchUpdatedAt(ctx context.Context, pipe redis.Pipeliner, baseKey string) {
	pipe.HSet(ctx, baseKey, workspaceFieldUpdatedAt, time.Now().Unix())
}
