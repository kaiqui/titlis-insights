package config

import "github.com/kelseyhightower/envconfig"

type Settings struct {
	Port           int    `envconfig:"PORT" default:"8091"`
	AppEnv         string `envconfig:"TITLIS_APP_ENV" default:"local"`
	LogLevel       string `envconfig:"LOG_LEVEL" default:"info"`
	LogFormat      string `envconfig:"LOG_FORMAT" default:"json"`
	InternalSecret string `envconfig:"INSIGHTS_INTERNAL_SECRET" default:"dev-secret"`

	TitlisAPIBaseURL        string `envconfig:"TITLIS_API_BASE_URL" default:"http://titlis-api:8080"`
	TitlisAPIInternalSecret string `envconfig:"TITLIS_API_INTERNAL_SECRET" default:"dev-secret"`

	DatadogSite              string `envconfig:"DATADOG_SITE" default:"datadoghq.com"`
	DatadogDefaultWindowDays int    `envconfig:"DATADOG_DEFAULT_WINDOW_DAYS" default:"30"`
	DatadogMinConfidence     string `envconfig:"DATADOG_MIN_CONFIDENCE" default:"0.7"`

	RecommendationCacheTTLMinutes int `envconfig:"INSIGHTS_RECOMMENDATION_CACHE_TTL_MINUTES" default:"360"`

	DatabaseURL string `envconfig:"DATABASE_URL" default:""`

	UseStubSource bool `envconfig:"INSIGHTS_USE_STUB_SOURCE" default:"true"`
}

func Load() (Settings, error) {
	var s Settings
	if err := envconfig.Process("", &s); err != nil {
		return Settings{}, err
	}
	return s, nil
}
