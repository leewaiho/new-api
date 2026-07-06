package model

import (
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	ModelInputModalityText  = "text"
	ModelInputModalityImage = "image"
)

type ModelMetadataList []string

type ModelDiscoveryMetadata struct {
	InputModalities  []string
	OutputModalities []string
	Capabilities     []string
	ContextLength    int
	MaxOutputTokens  int
}

func (l ModelMetadataList) Value() (driver.Value, error) {
	if len(l) == 0 {
		return "", nil
	}
	bytes, err := common.Marshal([]string(l))
	if err != nil {
		return nil, err
	}
	return string(bytes), nil
}

func (l *ModelMetadataList) Scan(value interface{}) error {
	if value == nil {
		*l = nil
		return nil
	}

	var raw string
	switch v := value.(type) {
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("unsupported model metadata list type %T", value)
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		*l = nil
		return nil
	}

	var values []string
	if err := common.UnmarshalJsonStr(raw, &values); err != nil {
		return err
	}
	*l = values
	return nil
}

func normalizeModelMetadataValues(values []string, allowed map[string]struct{}) []string {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if allowed != nil {
			if _, ok := allowed[value]; !ok {
				continue
			}
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func NormalizeInputModalities(values []string) []string {
	return normalizeModelMetadataValues(values, map[string]struct{}{
		ModelInputModalityText:  {},
		ModelInputModalityImage: {},
	})
}

func NormalizeModelMetadataList(values []string) []string {
	return normalizeModelMetadataValues(values, nil)
}

func (mi *Model) NormalizeMetadata() {
	mi.InputModalities = NormalizeInputModalities(mi.InputModalities)
	mi.OutputModalities = NormalizeModelMetadataList(mi.OutputModalities)
	mi.Capabilities = NormalizeModelMetadataList(mi.Capabilities)
}

func (mi *Model) DiscoveryMetadata() ModelDiscoveryMetadata {
	return ModelDiscoveryMetadata{
		InputModalities:  NormalizeInputModalities(mi.InputModalities),
		OutputModalities: NormalizeModelMetadataList(mi.OutputModalities),
		Capabilities:     NormalizeModelMetadataList(mi.Capabilities),
		ContextLength:    mi.ContextLength,
		MaxOutputTokens:  mi.MaxOutputTokens,
	}
}

func (m ModelDiscoveryMetadata) IsEmpty() bool {
	return len(m.InputModalities) == 0 && len(m.OutputModalities) == 0 && len(m.Capabilities) == 0 && m.ContextLength == 0 && m.MaxOutputTokens == 0
}
