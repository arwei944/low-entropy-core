//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import "fmt"

// TierBridge 表示一个兼容适配层桥接定义。
type TierBridge struct {
	Name        string
	Description string
	FromTier    ComplexityTier
	ToTier      ComplexityTier
	OldAPI      string
	NewAPI      string
	AdapterCode string
}

// GenerateTierBridge 为指定模块生成兼容层代码。
func GenerateTierBridge(fromTier, toTier ComplexityTier, module string) string {
	bridges := getStandardBridges()
	for _, b := range bridges {
		if b.FromTier <= fromTier && b.ToTier <= toTier && b.Name == module {
			return b.AdapterCode
		}
	}
	return fmt.Sprintf("// No standard bridge defined for module %s (%s → %s)\n", module, fromTier, toTier)
}

// ListAvailableBridges 列出所有可用的兼容层桥接。
func ListAvailableBridges(fromTier, toTier ComplexityTier) []TierBridge {
	all := getStandardBridges()
	var result []TierBridge
	for _, b := range all {
		if b.ToTier > fromTier && b.ToTier <= toTier {
			result = append(result, b)
		}
	}
	return result
}

func getStandardBridges() []TierBridge {
	return []TierBridge{
		{
			Name:        "eventstore",
			Description: "Bridge from inline event logging to EventStore",
			FromTier:    TierL1,
			ToTier:      TierL2,
			OldAPI:      "SaveEvent(id, type, data)",
			NewAPI:      "EventStore.Execute(ctx, envelope)",
			AdapterCode: `// EventStoreBridge adapts simple event logging to EventStore.
type EventStoreBridge struct {
	Store *EventStore
}

func NewEventStoreBridge(store *EventStore) *EventStoreBridge {
	return &EventStoreBridge{Store: store}
}

func (b *EventStoreBridge) SaveEvent(ctx context.Context, eventID, eventType string, data []byte) error {
	env := EventEnvelope{
		EventID:   eventID,
		EventType: eventType,
		EventData: data,
		Timestamp: time.Now(),
	}
	_, err := b.Store.Execute(ctx, env)
	return err
}`,
		},
		{
			Name:        "eventbus",
			Description: "Bridge from direct function calls to EventBus pub/sub",
			FromTier:    TierL1,
			ToTier:      TierL2,
			OldAPI:      "Direct function call",
			NewAPI:      "EventBus.Subscribe/Publish",
			AdapterCode: `// EventBusBridge adapts direct calls to EventBus pub/sub.
type EventBusBridge struct {
	Bus *EventBus
}

func NewEventBusBridge(bus *EventBus) *EventBusBridge {
	return &EventBusBridge{Bus: bus}
}

func (b *EventBusBridge) Notify(ctx context.Context, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	env := EventEnvelope{
		EventID:   uuid.New().String(),
		EventType: eventType,
		EventData: data,
		Timestamp: time.Now(),
	}
	_, err = b.Bus.Execute(ctx, env)
	return err
}`,
		},
		{
			Name:        "config",
			Description: "Bridge from env vars to structured AppConfig",
			FromTier:    TierL1,
			ToTier:      TierL2,
			OldAPI:      "os.Getenv()",
			NewAPI:      "LoadAppConfigFromFile() + ApplyEnvOverrides()",
			AdapterCode: `// ConfigBridge adapts environment variables to AppConfig.
type ConfigBridge struct {
	Config AppConfig
}

func NewConfigBridge() *ConfigBridge {
	return &ConfigBridge{Config: DefaultAppConfig()}
}

func (b *ConfigBridge) LoadFromEnv() {
	ApplyEnvOverrides(&b.Config)
}`,
		},
		{
			Name:        "security",
			Description: "Bridge from plain tokens to CapabilityToken",
			FromTier:    TierL2,
			ToTier:      TierL3,
			OldAPI:      "Plain string token",
			NewAPI:      "CapabilityToken.Sign/Verify",
			AdapterCode: `// SecurityBridge adapts plain tokens to CapabilityToken.
type SecurityBridge struct {
	SecretKey []byte
}

func NewSecurityBridge(secretKey []byte) *SecurityBridge {
	return &SecurityBridge{SecretKey: secretKey}
}

func (b *SecurityBridge) CreateToken(agentID string, capabilities []string) (string, error) {
	token := NewCapabilityToken(agentID, capabilities)
	token.Sign(b.SecretKey)
	return token.String(), nil
}

func (b *SecurityBridge) ValidateToken(tokenStr string) (*CapabilityToken, error) {
	token, err := ParseCapabilityToken(tokenStr)
	if err != nil {
		return nil, err
	}
	if !token.Verify(b.SecretKey) {
		return nil, fmt.Errorf("token verification failed")
	}
	return token, nil
}`,
		},
	}
}