package simulator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	NumUsers  int
	ServerURL string
	MinDelay  time.Duration
	MaxDelay  time.Duration
}

func DefaultConfig(serverURL string) Config {
	return Config{
		NumUsers:  1000,
		ServerURL: serverURL,
		MinDelay:  10 * time.Millisecond,
		MaxDelay:  1000 * time.Millisecond,
	}
}

type Result struct {
	Total         int
	CorrectSent   int64
	IncorrectSent int64
	Succeeded     int64
	Failed        int64
	Duration      time.Duration
}

func Run(cfg Config) Result {
	var (
		wg            sync.WaitGroup
		succeeded     atomic.Int64
		failed        atomic.Int64
		correctSent   atomic.Int64
		incorrectSent atomic.Int64
	)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        cfg.NumUsers,
			MaxIdleConnsPerHost: cfg.NumUsers,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	log.Printf("[Simulator] Spawning %d users against %s", cfg.NumUsers, cfg.ServerURL)
	start := time.Now()

	for i := 0; i < cfg.NumUsers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			userID := fmt.Sprintf("user-%05d", idx)
			correct := rand.Intn(2) == 1 // 50/50 random correct or wrong

			if correct {
				correctSent.Add(1)
			} else {
				incorrectSent.Add(1)
			}

			// Random delay simulates network lag (10ms–1000ms).
			delay := cfg.MinDelay + time.Duration(rand.Int63n(int64(cfg.MaxDelay-cfg.MinDelay)))
			time.Sleep(delay)

			if err := sendResponse(client, cfg.ServerURL, userID, correct); err != nil {
				log.Printf("[Simulator] %s error: %v", userID, err)
				failed.Add(1)
			} else {
				succeeded.Add(1)
			}
		}(i)
	}

	wg.Wait()

	return Result{
		Total:         cfg.NumUsers,
		CorrectSent:   correctSent.Load(),
		IncorrectSent: incorrectSent.Load(),
		Succeeded:     succeeded.Load(),
		Failed:        failed.Load(),
		Duration:      time.Since(start),
	}
}

func sendResponse(client *http.Client, baseURL, userID string, correct bool) error {
	body, _ := json.Marshal(map[string]interface{}{
		"user_id":        userID,
		"correct_answer": correct,
	})
	resp, err := client.Post(baseURL+"/submit", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
