package main

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"math"
	"math/rand"
	"sync"
	"time"
)

// JitterMode selects how the beacon interval is shaped.
type JitterMode string

const (
	// JitterUniform spreads the interval evenly around the base value.
	JitterUniform JitterMode = "uniform"
	// JitterActivity shapes the interval around the activity of the host.
	JitterActivity JitterMode = "activity"
	// JitterNone keeps the base interval untouched.
	JitterNone JitterMode = "none"
)

const (
	minJitterSeconds = 1
	maxJitterSeconds = 24 * 3600
)

// parseJitterMode validates a jitter mode name.
func parseJitterMode(raw string) (JitterMode, bool) {
	switch JitterMode(raw) {
	case JitterUniform:
		return JitterUniform, true
	case JitterActivity:
		return JitterActivity, true
	case JitterNone:
		return JitterNone, true
	}
	return JitterActivity, false
}

// jitterRand is a concurrency safe random source seeded from the system.
type jitterRand struct {
	mu  sync.Mutex
	rng *rand.Rand
}

func newJitterRand() *jitterRand {
	var seed [8]byte
	if _, err := cryptorand.Read(seed[:]); err != nil {
		seed = [8]byte{byte(time.Now().UnixNano())}
	}
	return &jitterRand{rng: rand.New(rand.NewSource(int64(binary.LittleEndian.Uint64(seed[:]))))}
}

func (j *jitterRand) float64() float64 {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.rng.Float64()
}

// NextInterval returns the number of milliseconds to wait before the next
// beacon. The result is always at least one second.
func NextInterval(baseMs int64, jitterPct uint32, mode JitterMode, now time.Time, rng *jitterRand) int64 {
	if baseMs <= 0 {
		baseMs = 10000
	}
	if jitterPct == 0 || mode == JitterNone {
		return clampMs(baseMs)
	}
	switch mode {
	case JitterActivity:
		return clampMs(int64(float64(baseMs) * activityFactor(now, rng)))
	default:
		return clampMs(int64(uniformInterval(baseMs, jitterPct, rng)))
	}
}

// uniformInterval spreads the interval evenly in [base*(1-p), base*(1+p)].
func uniformInterval(baseMs int64, jitterPct uint32, rng *jitterRand) float64 {
	pct := float64(jitterPct) / 100.0
	delta := (rng.float64()*2 - 1) * pct
	return float64(baseMs) * (1 + delta)
}

// activityFactor shapes the interval around the activity of the host: the
// beacon is faster during the working hours and slower at night, and the
// distribution has a right heavy tail so the pattern does not look uniform.
func activityFactor(now time.Time, rng *jitterRand) float64 {
	factor := diurnalFactor(now) * shapeFactor(rng.float64())
	if factor < 0.25 {
		factor = 0.25
	}
	if factor > 4.0 {
		factor = 4.0
	}
	return factor
}

// diurnalFactor returns 1 during the working hours and grows at night.
func diurnalFactor(now time.Time) float64 {
	hour := float64(now.Hour())
	switch {
	case hour >= 9 && hour < 18:
		return 1.0
	case hour >= 8 && hour < 9, hour >= 18 && hour < 20:
		return 1.35
	case hour >= 20 && hour < 23:
		return 1.8
	case hour >= 23 || hour < 6:
		return 2.4
	default:
		return 1.6
	}
}

// shapeControlPoints is a piecewise linear quantile with a right heavy tail.
var shapeControlPoints = []float64{
	0.00, 0.35, 0.70, 1.00, 1.55, 2.30, 3.20,
}

// shapeFactor maps a uniform sample onto the shaped distribution.
func shapeFactor(u float64) float64 {
	n := len(shapeControlPoints) - 1
	if u <= 0 {
		return shapeControlPoints[0]
	}
	if u >= 1 {
		return shapeControlPoints[n]
	}
	position := u * float64(n)
	index := int(position)
	if index >= n {
		return shapeControlPoints[n]
	}
	frac := position - float64(index)
	low := shapeControlPoints[index]
	high := shapeControlPoints[index+1]
	return low + (high-low)*frac
}

// backoffDuration returns the wait time after a failed beacon. The delay
// grows exponentially and is capped at five minutes.
func backoffDuration(baseSec uint32, failures int) time.Duration {
	if failures < 1 {
		failures = 1
	}
	if failures > 10 {
		failures = 10
	}
	seconds := float64(baseSec) * math.Pow(2, float64(failures-1))
	if seconds < 1 {
		seconds = 1
	}
	if seconds > 300 {
		seconds = 300
	}
	// A small random offset avoids a synchronised fleet of agents.
	offset := float64(time.Now().UnixNano()%1000) / 1000.0
	return time.Duration((seconds + offset) * float64(time.Second))
}

// clampMs keeps the interval inside the sane bounds.
func clampMs(ms int64) int64 {
	minMs := int64(minJitterSeconds * 1000)
	maxMs := int64(maxJitterSeconds * 1000)
	if ms < minMs {
		return minMs
	}
	if ms > maxMs {
		return maxMs
	}
	return ms
}
