package config

import (
	"fmt"
	"github.com/ilyakaznacheev/cleanenv"
)

type Crypto struct {
	Symbols    []string `yaml:"symbols"`
	MaxWorkers int      `yaml:"max_workers"`
	BinanceUrl string   `yaml:"binance_url"`
}

func New(path string) (*Crypto, error) {
	var crypto Crypto

	err := cleanenv.ReadConfig(path, &crypto)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	return &crypto, nil
}
