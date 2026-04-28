package eino

import (
	"context"
	"fmt"

	"cloud-agent-monitor/pkg/config"

	"github.com/cloudwego/eino-ext/components/model/openai"
	amodel "github.com/cloudwego/eino/components/model"
)

// NewChatModel creates an eino ToolCallingChatModel using the OpenAI-compatible provider.
// It supports any OpenAI API-compatible endpoint (OpenAI, DeepSeek, Azure, vLLM, etc.)
// via the BaseURL configuration field.
func NewChatModel(ctx context.Context, cfg config.LLMConfig) (amodel.ToolCallingChatModel, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("llm.api_key is required in config.yaml")
	}

	modelCfg := &openai.ChatModelConfig{
		APIKey: cfg.APIKey,
		Model:  cfg.Model,
	}

	if cfg.BaseURL != "" {
		modelCfg.BaseURL = cfg.BaseURL
	}

	chatModel, err := openai.NewChatModel(ctx, modelCfg)
	if err != nil {
		return nil, fmt.Errorf("create %s chat model (%s): %w", cfg.Provider, cfg.Model, err)
	}

	return chatModel, nil
}
