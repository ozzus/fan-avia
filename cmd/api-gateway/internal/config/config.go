package config

import (
	"flag"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env     string        `yaml:"env" env:"ENV" env-default:"local"`
	Log     LogConfig     `yaml:"log"`
	HTTP    HTTPConfig    `yaml:"http"`
	Clients ClientsConfig `yaml:"clients"`
	Jaeger  JaegerConfig  `yaml:"jaeger"`
}

type LogConfig struct {
	Level string `yaml:"level" env:"LOG_LEVEL" env-default:"info"`
}

type HTTPConfig struct {
	Host            string        `yaml:"host" env:"HTTP_HOST" env-default:"0.0.0.0"`
	Port            int           `yaml:"port" env:"HTTP_PORT" env-default:"8080"`
	ReadTimeout     time.Duration `yaml:"read_timeout" env:"HTTP_READ_TIMEOUT" env-default:"5s"`
	WriteTimeout    time.Duration `yaml:"write_timeout" env:"HTTP_WRITE_TIMEOUT" env-default:"5s"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" env:"HTTP_SHUTDOWN_TIMEOUT" env-default:"5s"`
}

type ClientsConfig struct {
	Airfare AirfareClientConfig `yaml:"airfare"`
	Match   MatchClientConfig   `yaml:"match"`
}

type AirfareClientConfig struct {
	Address      string        `yaml:"address" env:"AIRFARE_ADDRESS"`
	Timeout      time.Duration `yaml:"timeout" env:"AIRFARE_TIMEOUT" env-default:"5s"`
	RetriesCount int           `yaml:"retriesCount" env:"AIRFARE_RETRIES_COUNT" env-default:"3"`
}

type MatchClientConfig struct {
	Address      string        `yaml:"address" env:"MATCH_ADDRESS"`
	Timeout      time.Duration `yaml:"timeout" env:"MATCH_TIMEOUT" env-default:"5s"`
	RetriesCount int           `yaml:"retriesCount" env:"MATCH_RETRIES_COUNT" env-default:"3"`
}

type JaegerConfig struct {
	Address string `yaml:"address" env:"JAEGER_ADDRESS"`
}

func MustLoad() *Config {
	path := fetchConfigPath()
	if path == "" {
		panic("config path is empty")
	}
	return MustLoadByPath(path)
}

func MustLoadByPath(configPath string) *Config {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("config file does not exists: " + configPath)
	}

	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		panic("cannot read the config: " + err.Error())
	}

	return &cfg
}

func fetchConfigPath() string {
	var res string

	flag.StringVar(&res, "config", "", "path to config file")
	flag.Parse()

	if res == "" {
		res = os.Getenv("CONFIG_PATH")
	}

	if res == "" {
		res = "config/local.yaml"
	}

	return res
}
