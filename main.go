package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"game-engine/api"
	"game-engine/engine"
	"game-engine/simulator"
)

func main() {
	mode := flag.String("mode", "all", `Run mode: "all" | "server" | "simulate"`)
	addr := flag.String("addr", ":8080", "Address the API server listens on")
	numUsers := flag.Int("n", 1000, "Total number of simulated users")
	serverURL := flag.String("url", "http://localhost:8080", "API server base URL")
	flag.Parse()

	switch *mode {
	case "server":
		runServer(*addr)

	case "simulate":
		runSimulator(*serverURL, *numUsers)

	default:
		ge := engine.New(2048)
		srv := api.New(ge)

		go func() {
			if err := srv.Start(*addr); err != nil && err != http.ErrServerClosed {
				log.Fatalf("[Main] Server error: %v", err)
			}
		}()

		time.Sleep(100 * time.Millisecond)
		log.Printf("[Main] Server ready. Launching %d users.", *numUsers)
		runSimulator(*serverURL, *numUsers)

		select {
		case <-ge.Done():
			log.Println("[Main] Game over – winner has been declared.")
		case <-time.After(5 * time.Second):
			log.Println("[Main] Timed out waiting for a winner.")
		}
	}
}

func runServer(addr string) {
	ge := engine.New(2048)
	srv := api.New(ge)
	go func() {
		<-ge.Done()
		log.Println("[Main] Winner declared.")
	}()
	if err := srv.Start(addr); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[Main] Server error: %v", err)
	}
}

func runSimulator(serverURL string, n int) {
	cfg := simulator.DefaultConfig(serverURL)
	cfg.NumUsers = n
	result := simulator.Run(cfg)

	log.Printf("[Simulator] Done. Total=%d  ✅CorrectSent=%d  ❌WrongSent=%d  OK=%d  Err=%d  Duration=%s",
		result.Total,
		result.CorrectSent,
		result.IncorrectSent,
		result.Succeeded,
		result.Failed,
		result.Duration.Round(time.Millisecond),
	)
}
