package utility

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

const (
	GOROUTINE_WARNING_THRESHOLD  = 500
	GOROUTINE_CRITICAL_THRESHOLD = 1000
	GOROUTINE_CHECK_INTERVAL     = 30 * time.Second
)

type GoroutineMonitor struct {
	warningThreshold  int
	criticalThreshold int
	checkInterval     time.Duration
	stopChan          chan struct{}
	running           bool
}

func NewGoroutineMonitor() *GoroutineMonitor {
	return &GoroutineMonitor{
		warningThreshold:  GOROUTINE_WARNING_THRESHOLD,
		criticalThreshold: GOROUTINE_CRITICAL_THRESHOLD,
		checkInterval:     GOROUTINE_CHECK_INTERVAL,
		stopChan:          make(chan struct{}),
		running:           false,
	}
}

func (gm *GoroutineMonitor) StartMonitoring(ctx context.Context) {
	if gm.running {
		return
	}

	gm.running = true
	fmt.Println("🔍 [GOROUTINE-MONITOR] Starting goroutine monitoring")

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Goroutine monitor panic: %v\n", r)
			}
			fmt.Println("🔍 [GOROUTINE-MONITOR] Goroutine monitoring stopped")
		}()

		ticker := time.NewTicker(gm.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				fmt.Println("🔍 [GOROUTINE-MONITOR] Context cancelled, stopping monitoring")
				return
			case <-gm.stopChan:
				fmt.Println("🔍 [GOROUTINE-MONITOR] Stop signal received")
				return
			case <-ticker.C:
				gm.checkGoroutineCount()
			}
		}
	}()
}

func (gm *GoroutineMonitor) StopMonitoring() {
	if !gm.running {
		return
	}

	gm.running = false
	close(gm.stopChan)
}

func (gm *GoroutineMonitor) checkGoroutineCount() {
	count := runtime.NumGoroutine()

	switch {
	case count >= gm.criticalThreshold:
		fmt.Printf("🚨 [GOROUTINE-MONITOR] CRITICAL: Goroutine count %d exceeds critical threshold %d\n",
			count, gm.criticalThreshold)
		gm.printGoroutineStack()
	case count >= gm.warningThreshold:
		fmt.Printf("⚠️ [GOROUTINE-MONITOR] WARNING: Goroutine count %d exceeds warning threshold %d\n",
			count, gm.warningThreshold)
	default:
		fmt.Printf("✅ [GOROUTINE-MONITOR] Goroutine count: %d (healthy)\n", count)
	}
}

func (gm *GoroutineMonitor) printGoroutineStack() {
	buf := make([]byte, 1<<16)
	stackSize := runtime.Stack(buf, true)
	fmt.Printf("🔍 [GOROUTINE-MONITOR] Stack trace:\n%s\n", buf[:stackSize])
}

func (gm *GoroutineMonitor) GetCurrentCount() int {
	return runtime.NumGoroutine()
}
