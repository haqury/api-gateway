package config

type PartnerConfig struct {
	Name      string `yaml:"name"`
	URL       string `yaml:"url"`
	APIKey    string `yaml:"api_key"`
	Enabled   bool   `yaml:"enabled"`
	Priority  int    `yaml:"priority"`
	RateLimit int    `yaml:"rate_limit"` // запросов в секунду
}

type RoutingRule struct {
	Condition string `yaml:"condition"` // "client_id LIKE 'partner1_%'"
	Partner   string `yaml:"partner"`
}
