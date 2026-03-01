package config

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env             string                `yaml:"env" env:"ENV" env-default:"local"`
	Jaeger          string                `yaml:"jaeger" env:"JAEGER" env-default:"jaeger"`
	AirfareCacheTTL time.Duration         `yaml:"airfare_cache_ttl" env:"AIRFARE_CACHE_TTL" env-default:"30m"`
	Log             LogConfig             `yaml:"log"`
	GRPC            GRPCConfig            `yaml:"grpc"`
	DB              DBConfig              `yaml:"db"`
	Redis           RedisConfig           `yaml:"redis"`
	MatchAdapter    MatchAdapterConfig    `yaml:"match_adapter"`
	MatchDayWindows MatchDayWindowsConfig `yaml:"match_day_windows"`
	Travelpayouts   TravelpayoutsConfig   `yaml:"travelpayouts"`
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
	Host     string `yaml:"host" env:"DB_HOST"`
	Port     int    `yaml:"port" env:"DB_PORT"`
	User     string `yaml:"user" env:"DB_USER"`
	Password string `yaml:"password" env:"DB_PASSWORD"`
	Name     string `yaml:"name" env:"DB_NAME"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr" env:"REDIS_ADDR" env-default:"localhost:6379"`
	Password string `yaml:"password" env:"REDIS_PASSWORD"`
	DB       int    `yaml:"db" env:"REDIS_DB" env-default:"0"`
}

type MatchAdapterConfig struct {
	Host    string        `yaml:"host" env:"MATCH_ADAPTER_HOST" env-default:"localhost"`
	Port    int           `yaml:"port" env:"MATCH_ADAPTER_PORT" env-default:"44045"`
	Timeout time.Duration `yaml:"timeout" env:"MATCH_ADAPTER_TIMEOUT" env-default:"3s"`
}

type MatchDayWindowsConfig struct {
	OutStrictEarliestBefore time.Duration `yaml:"out_strict_earliest_before" env:"MATCH_DAY_OUT_STRICT_EARLIEST_BEFORE" env-default:"4h"`
	OutStrictLatestBefore   time.Duration `yaml:"out_strict_latest_before" env:"MATCH_DAY_OUT_STRICT_LATEST_BEFORE" env-default:"2h"`
	OutSoft1EarliestBefore  time.Duration `yaml:"out_soft1_earliest_before" env:"MATCH_DAY_OUT_SOFT1_EARLIEST_BEFORE" env-default:"8h"`
	OutSoft1LatestBefore    time.Duration `yaml:"out_soft1_latest_before" env:"MATCH_DAY_OUT_SOFT1_LATEST_BEFORE" env-default:"0h"`
	OutSoft2EarliestBefore  time.Duration `yaml:"out_soft2_earliest_before" env:"MATCH_DAY_OUT_SOFT2_EARLIEST_BEFORE" env-default:"24h"`
	OutSoft2LatestBefore    time.Duration `yaml:"out_soft2_latest_before" env:"MATCH_DAY_OUT_SOFT2_LATEST_BEFORE" env-default:"0h"`
	RetStrictNotBeforeAfter time.Duration `yaml:"ret_strict_not_before_after" env:"MATCH_DAY_RET_STRICT_NOT_BEFORE_AFTER" env-default:"4h"`
	RetSoft1NotBeforeAfter  time.Duration `yaml:"ret_soft1_not_before_after" env:"MATCH_DAY_RET_SOFT1_NOT_BEFORE_AFTER" env-default:"2h"`
	RetSoft2NotBeforeAfter  time.Duration `yaml:"ret_soft2_not_before_after" env:"MATCH_DAY_RET_SOFT2_NOT_BEFORE_AFTER" env-default:"0h"`
}

type TravelpayoutsConfig struct {
	BaseURL  string        `yaml:"base_url" env:"TRAVELPAYOUTS_BASE_URL" env-default:"https://api.travelpayouts.com"`
	Token    string        `yaml:"token" env:"TRAVELPAYOUTS_TOKEN"`
	Currency string        `yaml:"currency" env:"TRAVELPAYOUTS_CURRENCY" env-default:"rub"`
	Limit    int           `yaml:"limit" env:"TRAVELPAYOUTS_LIMIT" env-default:"30"`
	Timeout  time.Duration `yaml:"timeout" env:"TRAVELPAYOUTS_TIMEOUT" env-default:"5s"`
}

func (c MatchAdapterConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
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
