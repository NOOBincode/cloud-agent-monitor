package eino

import (
	"context"
	"testing"

	"cloud-agent-monitor/pkg/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChatModel_MissingAPIKey(t *testing.T) {
	_, err := NewChatModel(context.Background(), config.LLMConfig{
		Provider: "openai",
		Model:    "gpt-4o-mini",
		APIKey:   "",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key is required")
}

func TestNewChatModel_ValidConfig(t *testing.T) {
	model, err := NewChatModel(context.Background(), config.LLMConfig{
		Provider: "openai",
		Model:    "gpt-4o-mini",
		APIKey:   "sk-test-key-12345",
	})
	require.NoError(t, err)
	require.NotNil(t, model)
}

func TestNewChatModel_WithBaseURL(t *testing.T) {
	model, err := NewChatModel(context.Background(), config.LLMConfig{
		Provider: "openai",
		Model:    "deepseek-chat",
		APIKey:   "sk-9bc6ad60d20d484ea13bf6bd4c2514fb",
		BaseURL:  "https://api.deepseek.com",
	})
	require.NoError(t, err)
	require.NotNil(t, model)
}
