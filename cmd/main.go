package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/MamushevArup/binance-scrap/config"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

type TickerPrice struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

type Worker struct {
	Symbols        []string
	requestsCount  int
	previousPrices map[string]string
	consoleMutex   *sync.Mutex
	stopCh         chan struct{}
	doneCh         chan struct{}
}

type Answer struct {
	Symbol  string
	Price   string
	Changed bool
}

func (w *Worker) Run(ctx context.Context, wg *sync.WaitGroup, url string, printCh chan<- Answer) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			log.Printf(ctx.Err().Error())
		case <-w.stopCh:
			close(w.doneCh)
			return
		default:
			for _, symbol := range w.Symbols {
				select {
				case <-w.stopCh:
					close(w.doneCh)
					return
				default:
					reqUrl := fmt.Sprintf("%s?symbol=%s", url, symbol)
					resp, err := http.Get(reqUrl)
					if err != nil {
						log.Printf("Failed to create request for %s: %v", symbol, err)
						continue
					}
					w.consoleMutex.Lock()
					w.requestsCount++
					w.consoleMutex.Unlock()
					if err != nil {
						log.Printf("Failed to fetch price for %s: %v", symbol, err)
						continue
					}

					body, err := io.ReadAll(resp.Body)
					if err != nil {
						log.Printf("Failed to read response for %s: %v", symbol, err)
						resp.Body.Close()
						continue
					}
					resp.Body.Close()

					var price TickerPrice
					err = json.Unmarshal(body, &price)
					if err != nil {
						log.Printf("Failed to unmarshal JSON for %s: %v", symbol, err)
						continue
					}

					prevPrice, exists := w.previousPrices[symbol]
					w.previousPrices[symbol] = price.Price

					changed := exists && prevPrice != price.Price
					printCh <- Answer{Symbol: price.Symbol, Price: price.Price, Changed: changed}

				}
			}
			time.Sleep(1 * time.Second) // Пауза между запросами
		}
	}
}

func (w *Worker) GetRequestsCount() int {
	return w.requestsCount
}

const (
	path        = "config.yaml"
	StopCommand = "STOP"
	endpoint    = "/api/v3/ticker/price"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg, err := config.New(path)
	if err != nil {
		log.Fatalf("Failed to read cfg: %v", err)
	}

	if cfg.MaxWorkers > runtime.NumCPU() {
		cfg.MaxWorkers = runtime.NumCPU()
	}

	consoleMutex := &sync.Mutex{}
	stopCh := make(chan struct{})
	printCh := make(chan Answer)
	workers := createWorkers(cfg, consoleMutex, stopCh)

	var wg sync.WaitGroup

	for i := 0; i < cfg.MaxWorkers; i++ {
		wg.Add(1)
		go workers[i].Run(ctx, &wg, cfg.BinanceUrl+endpoint, printCh)
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				totalRequests := 0
				for _, worker := range workers {
					totalRequests += worker.GetRequestsCount()
				}
				consoleMutex.Lock()
				fmt.Printf("workers requests total: %d\n", totalRequests)
				consoleMutex.Unlock()
			case msg := <-printCh:
				if msg.Changed {
					fmt.Printf("%s price:%s changed\n", msg.Symbol, msg.Price)
				} else {
					fmt.Printf("%s price:%s\n", msg.Symbol, msg.Price)
				}
			}
		}
	}()

	reader := bufio.NewReader(os.Stdin)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("Failed to read input: %v", err)
		}
		if strings.EqualFold(strings.TrimSpace(input), StopCommand) {
			close(stopCh)
			break
		}
	}

	waitForShutdown(workers, 10*time.Second)
	wg.Wait()
}

// waitForShutdown ожидает завершения всех воркеров в течение заданного времени
func waitForShutdown(workers []*Worker, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	doneCh := make(chan struct{})
	go func() {
		for _, worker := range workers {
			<-worker.doneCh
		}
		close(doneCh)
	}()

	select {
	case <-ctx.Done():
		fmt.Println("Timeout reached, forcing shutdown")
	case <-doneCh:
		fmt.Println("All workers have shut down gracefully")
	}
}

func createWorkers(config *config.Crypto, consoleMutex *sync.Mutex, stopCh chan struct{}) []*Worker {
	workers := make([]*Worker, config.MaxWorkers)

	for i := 0; i < config.MaxWorkers; i++ {
		workers[i] = &Worker{
			Symbols:        []string{},
			previousPrices: make(map[string]string),
			consoleMutex:   consoleMutex,
			stopCh:         stopCh,
			doneCh:         make(chan struct{}),
		}
	}

	for i, symbol := range config.Symbols {
		workerIndex := i % config.MaxWorkers
		workers[workerIndex].Symbols = append(workers[workerIndex].Symbols, symbol)
	}

	return workers
}
