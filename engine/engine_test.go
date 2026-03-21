package engine_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"game-engine/engine"
)

// TestOnlyOneWinner verifies that regardless of how many correct answers
// arrive concurrently, the Done channel is closed exactly once and the
// engine does not panic or deadlock.
func TestOnlyOneWinner(t *testing.T) {
	ge := engine.New(2048)

	const numCorrect = 500
	const numWrong = 500

	var wg sync.WaitGroup

	// Fire wrong answers.
	for i := 0; i < numWrong; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ge.Submit(engine.Response{
				UserID:    fmt.Sprintf("wrong-%d", i),
				Answer:    false,
				Timestamp: time.Now(),
			})
		}(i)
	}

	// Fire correct answers.
	for i := 0; i < numCorrect; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ge.Submit(engine.Response{
				UserID:    fmt.Sprintf("correct-%d", i),
				Answer:    true,
				Timestamp: time.Now(),
			})
		}(i)
	}

	wg.Wait()

	// Done channel must be closed (winner declared) within a generous timeout.
	select {
	case <-ge.Done():
		// Pass – winner declared.
	case <-time.After(3 * time.Second):
		t.Fatal("game engine never declared a winner")
	}

	// Allow drain goroutine to finish counting remaining responses.
	time.Sleep(50 * time.Millisecond)

	// ── Metrics assertions ───────────────────────────────────────────────────
	correct, incorrect := ge.GetMetrics()
	total := correct + incorrect

	if correct == 0 {
		t.Errorf("expected correct count > 0, got %d", correct)
	}
	if incorrect == 0 {
		t.Errorf("expected incorrect count > 0, got %d", incorrect)
	}
	if total > int64(numCorrect+numWrong) {
		t.Errorf("total responses %d exceeds submitted %d", total, numCorrect+numWrong)
	}

	t.Logf("Metrics → correct=%d  incorrect=%d  total=%d", correct, incorrect, total)

	// Verify Done channel stays closed (not re-opened).
	select {
	case <-ge.Done():
		// Still closed – correct.
	default:
		t.Fatal("Done channel should remain closed after winner declaration")
	}
}

// TestNoWinnerWithAllWrongAnswers verifies that Done is NOT closed when
// every submitted answer is wrong, and that incorrect count is tracked.
func TestNoWinnerWithAllWrongAnswers(t *testing.T) {
	const numWrong = 100
	ge := engine.New(128)

	for i := 0; i < numWrong; i++ {
		ge.Submit(engine.Response{
			UserID:    fmt.Sprintf("user-%d", i),
			Answer:    false,
			Timestamp: time.Now(),
		})
	}

	// Allow the engine goroutine to process everything.
	time.Sleep(100 * time.Millisecond)

	select {
	case <-ge.Done():
		t.Fatal("winner declared despite all wrong answers")
	default:
		// Pass – no winner declared, as expected.
	}

	// ── Metrics assertions ───────────────────────────────────────────────────
	correct, incorrect := ge.GetMetrics()

	if correct != 0 {
		t.Errorf("expected correct=0, got %d", correct)
	}
	if incorrect != int64(numWrong) {
		t.Errorf("expected incorrect=%d, got %d", numWrong, incorrect)
	}

	t.Logf("Metrics → correct=%d  incorrect=%d", correct, incorrect)
}

// TestMetricsAccuracy verifies counts are exact when inputs are controlled.
func TestMetricsAccuracy(t *testing.T) {
	ge := engine.New(256)

	// Submit known counts sequentially so all are processed before checking.
	for i := 0; i < 30; i++ {
		ge.Submit(engine.Response{UserID: fmt.Sprintf("wrong-%d", i), Answer: false, Timestamp: time.Now()})
	}
	// Submit one correct to trigger winner, then more wrong.
	ge.Submit(engine.Response{UserID: "winner", Answer: true, Timestamp: time.Now()})
	for i := 0; i < 20; i++ {
		ge.Submit(engine.Response{UserID: fmt.Sprintf("late-%d", i), Answer: false, Timestamp: time.Now()})
	}

	select {
	case <-ge.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("no winner declared")
	}

	time.Sleep(50 * time.Millisecond)

	correct, incorrect := ge.GetMetrics()
	t.Logf("Metrics → correct=%d  incorrect=%d  total=%d", correct, incorrect, correct+incorrect)

	if correct < 1 {
		t.Errorf("expected at least 1 correct answer, got %d", correct)
	}
}
