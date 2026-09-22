package components

import (
	"fmt"
	"strings"
	"time"
)

// View renders the progress component
func (p *Progress) View() string {
	var s strings.Builder

	// Title
	s.WriteString(p.titleStyle.Render(p.title))
	s.WriteString("\n\n")

	if p.completed {
		// Show completion status
		if p.success {
			s.WriteString(p.completeStyle.Render("✓ " + p.message))
		} else {
			s.WriteString(p.errorStyle.Render("✗ " + p.message))
		}
	} else {
		// Show progress bar
		barWidth := 40
		if p.width > 0 && p.width < 60 {
			barWidth = p.width - 20
			if barWidth < 1 {
				barWidth = 1
			}
		}

		var percentage float64
		if p.total > 0 {
			percentage = float64(p.current) / float64(p.total)
			if percentage > 1 {
				percentage = 1
			}
		}

		filled := int(percentage * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

		s.WriteString(p.progressStyle.Render(fmt.Sprintf("[%s] %.1f%%", bar, percentage*100)))

		// Show size information if available
		if p.total > 0 {
			fmt.Fprintf(&s, " (%s / %s)", FormatBytes(p.current), FormatBytes(p.total))
		}

		// Show enhanced speed and ETA information
		elapsed := time.Since(p.startTime)
		if elapsed > time.Second && p.current > 0 {
			// Use average speed for more stable display
			displaySpeed := p.avgSpeed
			if displaySpeed == 0 {
				displaySpeed = float64(p.current) / elapsed.Seconds()
			}

			s.WriteString(" | ")
			s.WriteString(p.speedStyle.Render(fmt.Sprintf("%s/s", FormatBytes(int64(displaySpeed)))))

			// Show instantaneous speed if significantly different
			if p.speed > 0 && p.speed != displaySpeed {
				instantDiff := (p.speed - displaySpeed) / displaySpeed
				if instantDiff > 0.2 || instantDiff < -0.2 { // Show if >20% difference
					fmt.Fprintf(&s, " (now: %s/s)", FormatBytes(int64(p.speed)))
				}
			}

			// Enhanced ETA calculation
			if p.total > 0 && displaySpeed > 0 {
				remaining := float64(p.total-p.current) / displaySpeed
				eta := time.Duration(remaining) * time.Second

				// Format ETA nicely
				if eta > time.Hour {
					fmt.Fprintf(&s, " | ETA: %dh%dm", int(eta.Hours()), int(eta.Minutes())%60)
				} else if eta > time.Minute {
					fmt.Fprintf(&s, " | ETA: %dm%ds", int(eta.Minutes()), int(eta.Seconds())%60)
				} else {
					fmt.Fprintf(&s, " | ETA: %ds", int(eta.Seconds()))
				}
			}

			// Show elapsed time
			if elapsed > time.Minute {
				fmt.Fprintf(&s, " | Elapsed: %dm%ds", int(elapsed.Minutes()), int(elapsed.Seconds())%60)
			} else {
				fmt.Fprintf(&s, " | Elapsed: %ds", int(elapsed.Seconds()))
			}
		}
	}

	// Show message if available
	if p.message != "" && !p.completed {
		s.WriteString("\n")
		s.WriteString(p.messageStyle.Render(p.message))
	}

	return s.String()
}
