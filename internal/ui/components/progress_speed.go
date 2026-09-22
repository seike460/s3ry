package components

import (
	"fmt"
	"time"
)

// speedSample represents a speed measurement sample
type speedSample struct {
	timestamp time.Time
	bytes     int64
}

// addSpeedSample adds a new speed measurement sample
func (p *Progress) addSpeedSample(timestamp time.Time, bytes int64) {
	sample := speedSample{
		timestamp: timestamp,
		bytes:     bytes,
	}

	// Add sample and maintain max size
	p.samples = append(p.samples, sample)
	if len(p.samples) > p.maxSamples {
		p.samples = p.samples[1:]
	}
}

// calculateAverageSpeed calculates the average speed from recent samples
func (p *Progress) calculateAverageSpeed() {
	if len(p.samples) < 2 {
		return
	}

	// Calculate average speed over the sample period
	first := p.samples[0]
	last := p.samples[len(p.samples)-1]

	deltaTime := last.timestamp.Sub(first.timestamp).Seconds()
	deltaBytes := last.bytes - first.bytes

	if deltaTime > 0 && deltaBytes > 0 {
		p.avgSpeed = float64(deltaBytes) / deltaTime
	}
}

// GetCurrentSpeed returns the current instantaneous speed
func (p *Progress) GetCurrentSpeed() float64 {
	return p.speed
}

// GetAverageSpeed returns the average speed over recent samples
func (p *Progress) GetAverageSpeed() float64 {
	return p.avgSpeed
}

// FormatBytes formats byte count as human readable string
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
