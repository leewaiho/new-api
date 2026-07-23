package advancedcustom

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/go-redis/redis/v8"
)

var errAdvancedCustomResponsesToolStateNotFound = errors.New("advanced custom responses tool state not found or expired")

type advancedCustomResponsesToolStateScope struct {
	TokenID        int
	Route          string
	RequestedModel string
	UpstreamModel  string
}

type advancedCustomResponsesToolState struct {
	Output           []dto.ResponsesOutput                   `json:"output"`
	ToolNameMappings map[string]dto.ResponsesToolNameMapping `json:"tool_name_mappings,omitempty"`
}

type advancedCustomResponsesToolStateStore interface {
	Set(context.Context, string, string, time.Duration) error
	Get(context.Context, string) (string, error)
}

type advancedCustomResponsesToolStateRedisStore struct{}

func (advancedCustomResponsesToolStateRedisStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if !common.RedisEnabled || common.RDB == nil {
		return errors.New("Redis is unavailable")
	}
	return common.RDB.Set(ctx, key, value, ttl).Err()
}

func (advancedCustomResponsesToolStateRedisStore) Get(ctx context.Context, key string) (string, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return "", errors.New("Redis is unavailable")
	}
	return common.RDB.Get(ctx, key).Result()
}

type advancedCustomResponsesToolStateCache struct {
	store advancedCustomResponsesToolStateStore
	now   func() time.Time
}

func newAdvancedCustomResponsesToolStateCache(store advancedCustomResponsesToolStateStore) *advancedCustomResponsesToolStateCache {
	return &advancedCustomResponsesToolStateCache{store: store, now: time.Now}
}

var defaultAdvancedCustomResponsesToolStateCache = newAdvancedCustomResponsesToolStateCache(advancedCustomResponsesToolStateRedisStore{})

func (c *advancedCustomResponsesToolStateCache) Save(scope advancedCustomResponsesToolStateScope, responseID string, state advancedCustomResponsesToolState, ttl time.Duration) error {
	if c == nil || c.store == nil {
		return errors.New("advanced custom responses tool state cache is unavailable")
	}
	if strings.TrimSpace(responseID) == "" {
		return errors.New("advanced custom responses tool state response_id is required")
	}
	if ttl <= 0 {
		return errors.New("advanced custom responses tool state TTL must be positive")
	}
	if len(state.Output) == 0 {
		return nil
	}
	plaintext, err := common.Marshal(state)
	if err != nil {
		return err
	}
	ciphertext, err := encryptAdvancedCustomResponsesToolState(plaintext)
	if err != nil {
		return err
	}
	return c.store.Set(context.Background(), c.key(scope, responseID), ciphertext, ttl)
}

func (c *advancedCustomResponsesToolStateCache) Load(scope advancedCustomResponsesToolStateScope, responseID string) (advancedCustomResponsesToolState, error) {
	if c == nil || c.store == nil {
		return advancedCustomResponsesToolState{}, errors.New("advanced custom responses tool state cache is unavailable")
	}
	if strings.TrimSpace(responseID) == "" {
		return advancedCustomResponsesToolState{}, errAdvancedCustomResponsesToolStateNotFound
	}
	ciphertext, err := c.store.Get(context.Background(), c.key(scope, responseID))
	if errors.Is(err, redis.Nil) || errors.Is(err, errAdvancedCustomResponsesToolStateNotFound) {
		return advancedCustomResponsesToolState{}, errAdvancedCustomResponsesToolStateNotFound
	}
	if err != nil {
		return advancedCustomResponsesToolState{}, err
	}
	plaintext, err := decryptAdvancedCustomResponsesToolState(ciphertext)
	if err != nil {
		return advancedCustomResponsesToolState{}, fmt.Errorf("invalid advanced custom responses tool state: %w", err)
	}
	var state advancedCustomResponsesToolState
	if err := common.Unmarshal(plaintext, &state); err == nil && len(state.Output) > 0 {
		return state, nil
	}

	var legacyOutput []dto.ResponsesOutput
	if err := common.Unmarshal(plaintext, &legacyOutput); err != nil {
		return advancedCustomResponsesToolState{}, fmt.Errorf("invalid advanced custom responses tool state: %w", err)
	}
	if len(legacyOutput) == 0 {
		return advancedCustomResponsesToolState{}, errAdvancedCustomResponsesToolStateNotFound
	}
	return advancedCustomResponsesToolState{Output: legacyOutput}, nil
}

func (c *advancedCustomResponsesToolStateCache) key(scope advancedCustomResponsesToolStateScope, responseID string) string {
	identity := strings.Join([]string{
		fmt.Sprintf("token=%d", scope.TokenID),
		"route=" + scope.Route,
		"requested_model=" + scope.RequestedModel,
		"upstream_model=" + scope.UpstreamModel,
		"response_id=" + responseID,
	}, "\x1f")
	return "advanced-custom:responses-tool-state:" + common.GenerateHMAC(identity)
}

func advancedCustomResponsesToolStateKey() []byte {
	sum := sha256.Sum256([]byte(common.CryptoSecret + ":advanced-custom-responses-tool-state:v1"))
	return sum[:]
}

func encryptAdvancedCustomResponsesToolState(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(advancedCustomResponsesToolStateKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func decryptAdvancedCustomResponsesToolState(encoded string) ([]byte, error) {
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(advancedCustomResponsesToolStateKey())
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(payload) < gcm.NonceSize() {
		return nil, errors.New("ciphertext is too short")
	}
	return gcm.Open(nil, payload[:gcm.NonceSize()], payload[gcm.NonceSize():], nil)
}
