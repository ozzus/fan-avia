package config

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env         string            `yaml:"env" env:"ENV" env-default:"local"`
	Jaeger      string            `yaml:"jaeger" env:"JAEGER" env-default:"jaeger"`
	Log         LogConfig         `yaml:"log"`
	GRPC        GRPCConfig        `yaml:"grpc"`
	DB          DBConfig          `yaml:"db"`
	Premierliga PremierligaConfig `yaml:"premierliga"`
}

type LogConfig struct {
	Level string `yaml:"level" env:"LOG_LEVEL" env-default:"info"`
}

type GRPCConfig struct {
	Host    string        `yaml:"host" env:"GRPC_HOST"`
	Port    int           `yaml:"port" env:"GRPC_PORT"`
	Timeout time.Duration `yaml:"timeout" env:"GRPC_TIMEOUT"`
}

type DBConfig struct {
	DSN      string `yaml:"dsn" env:"DB_DSN"`
	Host     string `yaml:"host" env:"DB_HOST"`
	Port     int    `yaml:"port" env:"DB_PORT"`
	User     string `yaml:"user" env:"DB_USER"`
	Password string `yaml:"password" env:"DB_PASSWORD"`
	Name     string `yaml:"name" env:"DB_NAME"`
	SSLMode  string `yaml:"sslmode" env:"DB_SSLMODE" env-default:"require"`
}

func (c DBConfig) DatabaseURL() string {
	if c.DSN != "" {
		return c.DSN
	}

	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "require"
	}

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   c.Name,
	}

	q := u.Query()
	q.Set("sslmode", sslMode)
	u.RawQuery = q.Encode()

	return u.String()
}

type PremierligaConfig struct {
	BaseURL string        `yaml:"base_url" env:"PREMIERLIGA_BASE_URL"`
	Timeout time.Duration `yaml:"timeout" env:"PREMIERLIGA_TIMEOUT" env-default:"5s"`
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
