package main

import (
	"context"
	"github.com/MamushevArup/binance-scrap/config"
	"github.com/MamushevArup/binance-scrap/internal/parser"
	"log"
	"sync"
)

const (
	path = "config.yaml"
)

func main() {
	cfg, err := config.New(path)
	if err != nil {
		log.Fatalf("error reading config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	done := make(chan struct{})

	tracker := parser.NewTracker()
	wg.Add(1)
	go tracker.Run(cfg, ctx, cancel, &wg, done)

	wg.Wait()
}
