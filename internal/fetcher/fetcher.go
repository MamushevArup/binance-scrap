package fetcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Fetcher interface {
	Fetch(url string) (string, float64, error)
}

type Impl struct{}

func (f *Impl) Fetch(url string) (string, float64, error) {
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
