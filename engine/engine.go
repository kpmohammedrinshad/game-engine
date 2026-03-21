// Package engine implements the Game Engine responsible for evaluating
// user responses in real-time and declaring exactly one winner.
package engine

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Response represents a single user's submitted answer.
type Response struct {
	UserID    string    `json:"user_id"`
	Answer    bool      `json:"correct_answer"`
	Timestamp time.Time `json:"timestamp"`
}

// Metrics holds real-time counters updated atomically as responses arrive.
type Metrics struct {
	CorrectCount   atomic.Int64
	IncorrectCount atomic.Int64
}

// GameEngine evaluates responses concurrently and declares the first
// correct responder as the winner using channels for event-driven handling.
type GameEngine struct {
	// winnerDeclared is 0 initially; CAS to 1 to claim the winner slot.
	winnerDeclared atomic.Int32

	// startTime is recorded when the engine is created.
	startTime time.Time

	// responses is a buffered channel fed by the API layer.
	// A dedicated goroutine drains it so HTTP handlers never block.
	responses chan Response

	// done is closed once a winner has been declared.
	done chan struct{}

	// metrics tracks correct/incorrect answer counts in real-time.
	metrics Metrics
}

// New creates and starts a GameEngine ready to receive responses.
func New(bufferSize int) *GameEngine {
	ge := &GameEngine{
		responses: make(chan Response, bufferSize),
		done:      make(chan struct{}),
		startTime: time.Now(),
	}
	go ge.run()
	return ge
}

// Submit enqueues a response for evaluation.
// Returns immediately — evaluation is asynchronous via channel.
func (ge *GameEngine) Submit(r Response) {
	select {
	case ge.responses <- r:
	case <-ge.done:
		// Game already finished – ignore.
	}
}

// Done returns a channel that is closed when the game ends.
func (ge *GameEngine) Done() <-chan struct{} {
	return ge.done
}

// GetMetrics returns a snapshot of current correct/incorrect counts.
func (ge *GameEngine) GetMetrics() (correct, incorrect int64) {
	return ge.metrics.CorrectCount.Load(), ge.metrics.IncorrectCount.Load()
}

// run is the single goroutine that reads from the responses channel.
// Using one consumer means evaluation is strictly ordered by arrival
// with zero contention — fully event-driven via channel select.
func (ge *GameEngine) run() {
	for {
		select {
		case r := <-ge.responses:
			ge.evaluate(r)
		case <-ge.done:
			// Drain any remaining responses after game ends.
			for {
				select {
				case r := <-ge.responses:
					// Still count metrics even after winner found.
					if r.Answer {
						ge.metrics.CorrectCount.Add(1)
					} else {
						ge.metrics.IncorrectCount.Add(1)
					}
				default:
					return
				}
			}
		}
	}
}

// evaluate updates metrics and checks whether a response wins.
func (ge *GameEngine) evaluate(r Response) {
	// ── Update metrics atomically ────────────────────────────────────────────
	if r.Answer {
		ge.metrics.CorrectCount.Add(1)
	} else {
		ge.metrics.IncorrectCount.Add(1)
	}

	// ── Winner check ─────────────────────────────────────────────────────────
	if !r.Answer {
		return
	}

	// CompareAndSwap ensures exactly one goroutine can declare a winner.
	if ge.winnerDeclared.CompareAndSwap(0, 1) {
		elapsed := time.Since(ge.startTime)
		correct, incorrect := ge.GetMetrics()
		total := correct + incorrect

		fmt.Printf("\n╔══════════════════════════════════════════╗\n")
		fmt.Printf("║           🏆  WINNER DECLARED            ║\n")
		fmt.Printf("╠══════════════════════════════════════════╣\n")
		fmt.Printf("║  User ID      : %-25s ║\n", r.UserID)
		fmt.Printf("║  Answered At  : %-25s ║\n", r.Timestamp.Format("15:04:05.000"))
		fmt.Printf("║  Time to Win  : %-25s ║\n", elapsed.Round(time.Millisecond))
		fmt.Printf("╠══════════════════════════════════════════╣\n")
		fmt.Printf("║           📊  METRICS SNAPSHOT           ║\n")
		fmt.Printf("╠══════════════════════════════════════════╣\n")
		fmt.Printf("║  ✅ Correct    : %-25d ║\n", correct)
		fmt.Printf("║  ❌ Incorrect  : %-25d ║\n", incorrect)
		fmt.Printf("║  📨 Total Rcvd : %-25d ║\n", total)
		fmt.Printf("╚══════════════════════════════════════════╝\n\n")

		close(ge.done)
	}
}
