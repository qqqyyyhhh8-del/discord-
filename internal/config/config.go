package config

import (
	"encoding/json"
	"errors"
	"os"
)

type OpenAIConfig struct {
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`
	ChatModel  string `json:"chat_model"`
	EmbedModel string `json:"embed_model"`
}

type BotConfig struct {
	DiscordToken string `json:"discord_token"`
	SystemPrompt string `json:"system_prompt"`
}

type Config struct {
	OpenAI OpenAIConfig `json:"openai"`
	Bot    BotConfig    `json:"bot"`
}

func Load() (Config, error) {
	data, err := os.ReadFile("config.json")
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Bot.DiscordToken == "" {
		return Config{}, errors.New("config.json missing bot.discord_token")
	}
	if cfg.OpenAI.APIKey == "" {
		return Config{}, errors.New("config.json missing openai.api_key")
	}
	if cfg.OpenAI.BaseURL == "" {
		return Config{}, errors.New("config.json missing openai.base_url")
	}
	if cfg.OpenAI.ChatModel == "" {
		return Config{}, errors.New("config.json missing openai.chat_model")
	}
	if cfg.OpenAI.EmbedModel == "" {
		return Config{}, errors.New("config.json missing openai.embed_model")
	}
	if cfg.Bot.SystemPrompt == "" {
		cfg.Bot.SystemPrompt = "你是Discord聊天助手，回答清晰、友好，避免重复。"
	}
	return cfg, nil
}
