package components

import (
	"fmt"
	"strings"
	"time"
)

const (
	// defaultBarWidth is the progress bar width on wide terminals.
	defaultBarWidth = 40
	// compactWidth is the terminal width below which the bar shrinks.
	compactWidth = 60
	// compactBarMargin reserves cells for the percentage and labels.
	compactBarMargin = 20
	// speedDiffThreshold shows the instantaneous speed alongside the average
	// when they differ by more than this fraction.
	speedDiffThreshold = 0.2
)

// View renders the progress component
func (p *Progress) View() string {
	var s strings.Builder

	s.WriteString(p.titleStyle.Render(p.title))
	s.WriteString("\n\n")

	if p.completed {
		p.renderResult(&s)
	} else {
		p.renderBar(&s)
	}

	if p.message != "" && !p.completed {
		s.WriteString("\n")
		s.WriteString(p.messageStyle.Render(p.message))
	}

	return s.String()
}

// renderResult renders the final success or failure line.
func (p *Progress) renderResult(s *strings.Builder) {
	if p.success {
		s.WriteString(p.completeStyle.Render("✓ " + p.message))
	} else {
		s.WriteString(p.errorStyle.Render("✗ " + p.message))
	}
}

// renderBar renders the progress bar with size and transfer statistics.
func (p *Progress) renderBar(s *strings.Builder) {
	barWidth := p.barWidth()
	percentage := p.percentage()
	filled := int(percentage * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	s.WriteString(p.progressStyle.Render(fmt.Sprintf("[%s] %.1f%%", bar, percentage*100)))
	if p.total > 0 {
		fmt.Fprintf(s, " (%s / %s)", FormatBytes(p.current), FormatBytes(p.total))
	}

	elapsed := time.Since(p.startTime)
	if elapsed > time.Second && p.current > 0 {
		p.renderStats(s, elapsed)
	}
}

// barWidth picks the bar width for the terminal width.
func (p *Progress) barWidth() int {
	if p.width <= 0 || p.width >= compactWidth {
		return defaultBarWidth
	}
	if w := p.width - compactBarMargin; w > 1 {
		return w
	}
	return 1
}

// percentage returns the completion fraction clamped to at most 1.
func (p *Progress) percentage() float64 {
	if p.total <= 0 {
		return 0
	}
	if pct := float64(p.current) / float64(p.total); pct < 1 {
		return pct
	}
	return 1
}

// renderStats renders speed, ETA, and elapsed time.
func (p *Progress) renderStats(s *strings.Builder, elapsed time.Duration) {
	// Use average speed for more stable display
	displaySpeed := p.avgSpeed
	if displaySpeed == 0 {
		displaySpeed = float64(p.current) / elapsed.Seconds()
	}

	s.WriteString(" | ")
	s.WriteString(p.speedStyle.Render(fmt.Sprintf("%s/s", FormatBytes(int64(displaySpeed)))))

	p.renderInstantSpeed(s, displaySpeed)
	p.renderETA(s, displaySpeed)
	p.renderElapsed(s, elapsed)
}

// renderInstantSpeed adds the instantaneous speed when it diverges from the
// displayed average.
func (p *Progress) renderInstantSpeed(s *strings.Builder, displaySpeed float64) {
	if p.speed <= 0 || p.speed == displaySpeed {
		return
	}
	instantDiff := (p.speed - displaySpeed) / displaySpeed
	if instantDiff > speedDiffThreshold || instantDiff < -speedDiffThreshold {
		fmt.Fprintf(s, " (now: %s/s)", FormatBytes(int64(p.speed)))
	}
}

// renderETA adds the estimated remaining time.
func (p *Progress) renderETA(s *strings.Builder, displaySpeed float64) {
	if p.total <= 0 || displaySpeed <= 0 {
		return
	}
	eta := time.Duration(float64(p.total-p.current)/displaySpeed) * time.Second
	fmt.Fprintf(s, " | ETA: %s", formatETA(eta))
}

// renderElapsed adds the elapsed transfer time.
func (p *Progress) renderElapsed(s *strings.Builder, elapsed time.Duration) {
	if elapsed > time.Minute {
		fmt.Fprintf(s, " | Elapsed: %dm%ds", int(elapsed.Minutes()), int(elapsed.Seconds())%60)
	} else {
		fmt.Fprintf(s, " | Elapsed: %ds", int(elapsed.Seconds()))
	}
}

// formatETA renders a duration compactly as h/m, m/s, or s.
func formatETA(d time.Duration) string {
	switch {
	case d > time.Hour:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	case d > time.Minute:
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
}
