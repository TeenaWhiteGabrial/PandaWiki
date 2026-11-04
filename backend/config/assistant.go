package config

import (
	"os"
	"strconv"
	"strings"
)

type AssistantConfig struct {
	GeneratorURL   string `mapstructure:"generator_url"`
	GeneratorToken string `mapstructure:"generator_token"`
	SearchKey      string `mapstructure:"search_key"`
	RetentionDays  int    `mapstructure:"retention_days"`
}

func (c *AssistantConfig) ApplyEnvironment() {
	if v := os.Getenv("DOCUMENT_GENERATOR_URL"); v != "" {
		c.GeneratorURL = v
	}
	if c.GeneratorURL == "" {
		c.GeneratorURL = "http://document-generator:8091"
	}
	c.GeneratorURL = strings.TrimRight(c.GeneratorURL, "/")
	if v := os.Getenv("DOCUMENT_GENERATOR_TOKEN"); v != "" {
		c.GeneratorToken = v
	}
	if v := os.Getenv("ASSISTANT_SEARCH_API_KEY"); v != "" {
		c.SearchKey = v
	}
	if v, e := strconv.Atoi(os.Getenv("ASSISTANT_RETENTION_DAYS")); e == nil && v > 0 && v <= 365 {
		c.RetentionDays = v
	}
	if c.RetentionDays <= 0 {
		c.RetentionDays = 30
	}
}
