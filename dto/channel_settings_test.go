package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvancedCustomValidateResponsesToChatConverterPath(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
			},
		},
	}
	require.NoError(t, valid.Validate())

	tests := []struct {
		name         string
		incomingPath string
	}{
		{name: "chat completions", incomingPath: "/v1/chat/completions"},
		{name: "responses compact", incomingPath: "/v1/responses/compact"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AdvancedCustomConfig{
				Routes: []AdvancedCustomRoute{
					{
						IncomingPath: tt.incomingPath,
						UpstreamPath: "/v1/chat/completions",
						Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
					},
				},
			}
			err := config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "converter does not match incoming_path")
		})
	}
}

func TestAdvancedCustomValidateResponsesToolsMode(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
				ConverterOptions: &AdvancedCustomConverterOptions{
					ResponsesToolsMode: AdvancedCustomResponsesToolsModePreserve,
				},
			},
		},
	}
	require.NoError(t, valid.Validate())

	invalidMode := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
				ConverterOptions: &AdvancedCustomConverterOptions{
					ResponsesToolsMode: "bad",
				},
			},
		},
	}
	require.ErrorContains(t, invalidMode.Validate(), "responses_tools_mode is invalid")

	wrongConverter := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "/v1/chat/completions",
				Converter:    AdvancedCustomConverterNone,
				ConverterOptions: &AdvancedCustomConverterOptions{
					ResponsesToolsMode: AdvancedCustomResponsesToolsModePreserve,
				},
			},
		},
	}
	require.ErrorContains(t, wrongConverter.Validate(), "only supported")
}
