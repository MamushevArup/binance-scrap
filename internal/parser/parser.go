package parser

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/MamushevArup/binance-scrap/config"
	"github.com/MamushevArup/binance-scrap/internal/price"
	"github.com/MamushevArup/binance-scrap/internal/tracker"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const apiUrl = "/api/v3/ticker/price"

type Tracker struct {
	counter *tracker.RequestCounter
}

type Task struct {
	URL string
}

type Result struct {
	Symbol string
	Price  float64
	Error  error
}

func NewTracker() *Tracker {
	return &Tracker{
		counter: tracker.NewCounter(),
	}
}

func (t *Tracker) GetRequestCount() int {
	return t.counter.GetRequestCount()
}

func (t *Tracker) Run(cfg *config.Crypto, ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup, done chan struct{}) {
	defer wg.Done()
	tasks := make(chan Task, cfg.MaxWorkers)
	results := make(chan Result, len(cfg.Symbols))
	priceTracker := price.NewPriceTracker()

	for i := 0; i < cfg.MaxWorkers; i++ {
		wg.Add(1)
		go worker(ctx, wg, tasks, results, done)
	}

	wg.Add(1)
	go writer(ctx, results, priceTracker, wg, done)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			for _, symbol := range cfg.Symbols {
				select {
				case <-ctx.Done():
					close(tasks)
					return
				case <-done:
					close(tasks)
					return
				case tasks <- Task{URL: fmt.Sprintf("%s%s?symbol=%s", cfg.BinanceUrl, apiUrl, symbol)}:
					t.counter.IncrementRequests()
				}
			}
			time.Sleep(1 * time.Second) // Delay to prevent hammering the API
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				fmt.Printf("Total requests: %d\n", t.GetRequestCount())
			}
		}
	}()

	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			default:
				input, _ := reader.ReadString('\n')
				if strings.TrimSpace(strings.ToUpper(input)) == "STOP" {
					fmt.Println("Received STOP command. Shutting down...")
					done <- struct{}{}
					cancel()
					return
				}
			}
		}
	}()
}

func fetch(url string) (string, float64, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", 0, fmt.Errorf("failed to fetch data from %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read response from %s: %w", url, err)
	}

	var result struct {
		Symbol string  `json:"symbol"`
		Price  float64 `json:"price,string"`
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", 0, fmt.Errorf("failed to unmarshal response from %s: %w", url, err)
	}

	return result.Symbol, result.Price, nil
}

func worker(ctx context.Context, wg *sync.WaitGroup, tasks <-chan Task, results chan<- Result, done <-chan struct{}) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case task, ok := <-tasks:
			if !ok {
				return
			}
			symbol, currPrice, err := fetch(task.URL)
			results <- Result{Symbol: symbol, Price: currPrice, Error: err}
		}
	}
}

func writer(ctx context.Context, results <-chan Result, priceTracker *price.Tracker, wg *sync.WaitGroup, done <-chan struct{}) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case result, ok := <-results:
			if !ok {
				return
			}
			if result.Error != nil {
				fmt.Printf("Error: %v\n", result.Error)
			} else {
				priceChanged := priceTracker.UpdatePrice(result.Symbol, result.Price)
				if priceChanged {
					fmt.Printf("Symbol: %s, Price: %.4f changed \n", result.Symbol, result.Price)
				} else {
					fmt.Printf("Symbol: %s, Price: %.4f\n", result.Symbol, result.Price)
				}
			}
		}
	}
}
