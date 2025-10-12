package promptinfra

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	promptdomain "electron-go-app/backend/internal/domain/prompt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const defaultQueueKey = "prompt:persistence:queue"

// PersistenceQueue 使用 Redis List 承载异步落库任务。
type PersistenceQueue struct {
	client *redis.Client
	key    string
}

// NewPersistenceQueue 构造队列。
func NewPersistenceQueue(client *redis.Client, key string) *PersistenceQueue {
	if key == "" {
		key = defaultQueueKey
	}
	return &PersistenceQueue{
		client: client,
		key:    key,
	}
}

// Enqueue 将任务编码后推入队列末尾。
func (q *PersistenceQueue) Enqueue(ctx context.Context, task promptdomain.PersistenceTask) (string, error) {
	if q == nil || q.client == nil {
		return "", fmt.Errorf("persistence queue not initialised")
	}
	if task.TaskID == "" {
		task.TaskID = uuid.NewString()
	}
	if task.RequestedAt.IsZero() {
		task.RequestedAt = time.Now()
	}
	payload, err := json.Marshal(task)
	if err != nil {
		return "", fmt.Errorf("encode persistence task: %w", err)
	}
	if err := q.client.RPush(ctx, q.key, payload).Err(); err != nil {
		return "", fmt.Errorf("enqueue persistence task: %w", err)
	}
	return task.TaskID, nil
}

// BlockingPop 阻塞式弹出一个任务，timeout 为 0 表示阻塞等待。
func (q *PersistenceQueue) BlockingPop(ctx context.Context, timeout time.Duration) (promptdomain.PersistenceTask, error) {
	if q == nil || q.client == nil {
		return promptdomain.PersistenceTask{}, fmt.Errorf("persistence queue not initialised")
	}
	var (
		result promptdomain.PersistenceTask
	)
	values, err := q.client.BLPop(ctx, timeout, q.key).Result()
	if err != nil {
		return result, err
	}
	if len(values) != 2 {
		return result, fmt.Errorf("unexpected blpop response: %#v", values)
	}
	if err := json.Unmarshal([]byte(values[1]), &result); err != nil {
		return promptdomain.PersistenceTask{}, fmt.Errorf("decode persistence task: %w", err)
	}
	return result, nil
}
