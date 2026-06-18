package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// persistentSubscription 持久化的订阅元数据。
// 注意：Handler 是函数，无法序列化。持久化仅保存元数据用于恢复审计。
type persistentSubscription struct {
	ID        string `json:"id"`
	EventType string `json:"event_type"`
	Async     bool   `json:"async"`
}

// PersistentEventBus 带持久化订阅元数据的 EventBus。
// 在 EventBus 基础上包装 StorageBackend，Subscribe/Unsubscribe 时自动持久化订阅元数据。
// 启动时从后端恢复订阅元数据（仅日志记录，实际 handler 需重新注册）。
type PersistentEventBus struct {
	bus     *EventBus
	backend StorageBackend
}

// NewPersistentEventBus 创建持久化 EventBus，自动从后端恢复元数据。
func NewPersistentEventBus(backend StorageBackend) (*PersistentEventBus, error) {
	peb := &PersistentEventBus{
		bus:     NewEventBus(),
		backend: backend,
	}
	if err := peb.restoreMetadata(context.Background()); err != nil {
		return nil, fmt.Errorf("persistent eventbus: restore: %w", err)
	}
	return peb, nil
}

// restoreMetadata 从持久化后端恢复订阅元数据（仅日志记录）。
func (peb *PersistentEventBus) restoreMetadata(ctx context.Context) error {
	keys, err := peb.backend.List(ctx, "subscriptions/")
	if err != nil {
		return err
	}
	for _, key := range keys {
		data, err := peb.backend.Load(ctx, key)
		if err != nil {
			continue
		}
		var sub persistentSubscription
		if err := json.Unmarshal(data, &sub); err != nil {
			continue
		}
		log.Printf("[eventbus] restored subscription: %s -> %s (async=%v)", sub.EventType, sub.ID, sub.Async)
	}
	if len(keys) > 0 {
		log.Printf("[eventbus] restored %d subscription metadata records (handlers must be re-registered)", len(keys))
	}
	return nil
}

// Subscribe 注册事件处理器并持久化元数据。
func (peb *PersistentEventBus) Subscribe(eventType string, handler EventHandler) *Subscription {
	sub := peb.bus.Subscribe(eventType, handler)
	peb.persistSubscription(sub, false)
	return sub
}

// SubscribeAsync 注册异步事件处理器并持久化元数据。
func (peb *PersistentEventBus) SubscribeAsync(eventType string, handler EventHandler) *Subscription {
	sub := peb.bus.SubscribeAsync(eventType, handler)
	peb.persistSubscription(sub, true)
	return sub
}

// Unsubscribe 取消订阅并删除持久化元数据。
func (peb *PersistentEventBus) Unsubscribe(subscriptionID string) {
	peb.bus.Unsubscribe(subscriptionID)
	key := peb.subscriptionKey(subscriptionID)
	peb.backend.Delete(context.Background(), key)
}

// Execute 发布事件到所有匹配的订阅者。
func (peb *PersistentEventBus) Execute(ctx context.Context, input EventEnvelope) (PublishResult, error) {
	return peb.bus.Execute(ctx, input)
}

// SubscriberCount 返回指定事件类型的订阅者数量。
func (peb *PersistentEventBus) SubscriberCount(eventType string) int {
	return peb.bus.SubscriberCount(eventType)
}

// subscriptionKey 生成订阅元数据的存储 key。
func (peb *PersistentEventBus) subscriptionKey(subscriptionID string) string {
	return "subscriptions/" + subscriptionID
}

// persistSubscription 持久化订阅元数据。
func (peb *PersistentEventBus) persistSubscription(sub *Subscription, async bool) {
	ps := persistentSubscription{
		ID:        sub.ID,
		EventType: sub.EventType,
		Async:     async,
	}
	data, err := json.Marshal(ps)
	if err != nil {
		return
	}
	peb.backend.Save(context.Background(), peb.subscriptionKey(sub.ID), data)
}

// RestoredSubscriptions 返回从持久化后端恢复的订阅元数据列表。
// 调用方可根据此列表重新注册 handler。
func (peb *PersistentEventBus) RestoredSubscriptions(ctx context.Context) ([]persistentSubscription, error) {
	keys, err := peb.backend.List(ctx, "subscriptions/")
	if err != nil {
		return nil, err
	}
	var subs []persistentSubscription
	for _, key := range keys {
		data, err := peb.backend.Load(ctx, key)
		if err != nil {
			continue
		}
		var sub persistentSubscription
		if err := json.Unmarshal(data, &sub); err != nil {
			continue
		}
		subs = append(subs, sub)
	}
	return subs, nil
}