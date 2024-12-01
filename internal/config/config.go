package config

import (
	"fmt"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Env             string `default:"production" yaml:"env"`
	LogLevel        string `default:"info" split_words:"true" yaml:"log_level"`
	ApiListenUrl    URL    `default:"tcp://0.0.0.0:4117" split_words:"true" yaml:"api_listen_addr"`
	ApiAdvertiseUrl URL    `default:"tcp://127.0.0.1:4117" split_words:"true" yaml:"api_advertise_url"`
	MultiCore       bool   `default:"false" split_words:"true" yaml:"multi_core"`
	WorkerPool      bool   `default:"false" split_words:"true" yaml:"worker_pool"`
	Cluster         ClusterConfig
	Metrics         MetricsConfig
}

type ClusterConfig struct {
	Enabled       bool          `envconfig:"RC_CLUSTER_ENABLED" default:"false" yaml:"enabled"`
	Url           URL           `envconfig:"RC_CLUSTER_URL" yaml:"url"`
	PeerUrls      []URL         `envconfig:"RC_CLUSTER_PEER_URLS" yaml:"peer_urls"`
	StateDir      string        `envconfig:"RC_CLUSTER_STATE_DIR" yaml:"state_dir"`
	CheckInterval time.Duration `envconfig:"RC_CLUSTER_CHECK_INTERVAL" default:"1s" yaml:"check_interval"`
}

type MetricsConfig struct {
	Enabled bool `envconfig:"RC_METRICS_ENABLED" default:"false" yaml:"enabled"`
}

var Default *Config

func Init() error {
	_ = godotenv.Load()
	var cfg Config
	if err := envconfig.Process("RC", &cfg); err != nil {
		return err
	}

	if cfg.Cluster.Enabled {
		url := &cfg.Cluster.Url
		if len(url.String()) == 0 {
			return fmt.Errorf("Invalid cluster.url: %s", url.String())
		}
	}
	Default = &cfg

	return nil
}

func CloneDefault() *Config {
	var cfg Config = *Default
	return &cfg
}

func GetDefault() *Config {
	return Default
}

func (cfg *Config) IsProdEnv() bool {
	return cfg.Env == "production"
}

func (cfg *Config) IsDevEnv() bool {
	return cfg.Env == "development"
}

func (cfg *Config) IsTestEnv() bool {
	return cfg.Env == "test"
}

func (cfg *Config) IsBenchmarkEnv() bool {
	return cfg.Env == "benchmark"
}

func (cfg *Config) ClusterNodeName() string {
	return cfg.Cluster.Url.NodeName()
}

func (cfg *Config) ClusterNodeID() uint32 {
	return cfg.Cluster.Url.NodeID()
}
