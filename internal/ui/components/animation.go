package components

import (
	"math"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AnimationType represents different types of animations
type AnimationType int

const (
	AnimationFadeIn AnimationType = iota
	AnimationFadeOut
	AnimationSlideIn
	AnimationSlideOut
	AnimationBounce
	AnimationPulse
)

// AnimationState represents the current state of an animation
type AnimationState int

const (
	AnimationStateIdle AnimationState = iota
	AnimationStateRunning
	AnimationStateCompleted
)

// AnimationTickMsg represents an animation tick message
type AnimationTickMsg struct {
	AnimationID string
	Frame       int
	Progress    float64
}

// Animation represents a UI animation
type Animation struct {
	id           string
	animationType AnimationType
	duration     time.Duration
	startTime    time.Time
	state        AnimationState
	progress     float64
	frames       int
	currentFrame int
	
	// Animation parameters
	startOpacity float64
	endOpacity   float64
	startOffset  int
	endOffset    int
	
	// Easing function
	easing func(float64) float64
	
	// Callback when animation completes
	onComplete func()
}

// AnimationManager manages multiple animations
type AnimationManager struct {
	animations map[string]*Animation
	styles     map[string]lipgloss.Style
}

// NewAnimationManager creates a new animation manager
func NewAnimationManager() *AnimationManager {
	return &AnimationManager{
		animations: make(map[string]*Animation),
		styles:     make(map[string]lipgloss.Style),
	}
}

// CreateFadeInAnimation creates a fade-in animation
func (am *AnimationManager) CreateFadeInAnimation(id string, duration time.Duration) *Animation {
	return am.createAnimation(id, AnimationFadeIn, duration, 0.0, 1.0, 0, 0)
}

// CreateFadeOutAnimation creates a fade-out animation
func (am *AnimationManager) CreateFadeOutAnimation(id string, duration time.Duration) *Animation {
	return am.createAnimation(id, AnimationFadeOut, duration, 1.0, 0.0, 0, 0)
}

// CreateSlideInAnimation creates a slide-in animation
func (am *AnimationManager) CreateSlideInAnimation(id string, duration time.Duration, startOffset int) *Animation {
	return am.createAnimation(id, AnimationSlideIn, duration, 1.0, 1.0, startOffset, 0)
}

// CreateSlideOutAnimation creates a slide-out animation
func (am *AnimationManager) CreateSlideOutAnimation(id string, duration time.Duration, endOffset int) *Animation {
	return am.createAnimation(id, AnimationSlideOut, duration, 1.0, 1.0, 0, endOffset)
}

// CreateBounceAnimation creates a bounce animation
func (am *AnimationManager) CreateBounceAnimation(id string, duration time.Duration) *Animation {
	anim := am.createAnimation(id, AnimationBounce, duration, 1.0, 1.0, 0, 0)
	anim.easing = bounceEasing
	return anim
}

// CreatePulseAnimation creates a pulse animation
func (am *AnimationManager) CreatePulseAnimation(id string, duration time.Duration) *Animation {
	anim := am.createAnimation(id, AnimationPulse, duration, 1.0, 1.0, 0, 0)
	anim.easing = pulseEasing
	return anim
}

// createAnimation creates a base animation
func (am *AnimationManager) createAnimation(id string, animType AnimationType, duration time.Duration, startOpacity, endOpacity float64, startOffset, endOffset int) *Animation {
	anim := &Animation{
		id:           id,
		animationType: animType,
		duration:     duration,
		state:        AnimationStateIdle,
		startOpacity: startOpacity,
		endOpacity:   endOpacity,
		startOffset:  startOffset,
		endOffset:    endOffset,
		frames:       int(duration.Milliseconds() / 16), // ~60fps
		easing:       easeInOutCubic,
	}
	
	am.animations[id] = anim
	return anim
}

// StartAnimation starts an animation
func (am *AnimationManager) StartAnimation(id string) tea.Cmd {
	anim, exists := am.animations[id]
	if !exists {
		return nil
	}
	
	anim.state = AnimationStateRunning
	anim.startTime = time.Now()
	anim.currentFrame = 0
	anim.progress = 0.0
	
	return am.animationTick(id)
}

// StopAnimation stops an animation
func (am *AnimationManager) StopAnimation(id string) {
	if anim, exists := am.animations[id]; exists {
		anim.state = AnimationStateIdle
		anim.progress = 0.0
		anim.currentFrame = 0
	}
}

// IsRunning checks if an animation is running
func (am *AnimationManager) IsRunning(id string) bool {
	if anim, exists := am.animations[id]; exists {
		return anim.state == AnimationStateRunning
	}
	return false
}

// GetProgress returns the current progress of an animation
func (am *AnimationManager) GetProgress(id string) float64 {
	if anim, exists := am.animations[id]; exists {
		return anim.progress
	}
	return 0.0
}

// ApplyAnimationStyle applies animation effects to a style
func (am *AnimationManager) ApplyAnimationStyle(id string, baseStyle lipgloss.Style, content string) string {
	anim, exists := am.animations[id]
	if !exists || anim.state != AnimationStateRunning {
		return baseStyle.Render(content)
	}
	
	style := baseStyle
	easedProgress := anim.easing(anim.progress)
	
	switch anim.animationType {
	case AnimationFadeIn, AnimationFadeOut:
		opacity := lerp(anim.startOpacity, anim.endOpacity, easedProgress)
		alpha := int(opacity * 255)
		if alpha < 0 {
			alpha = 0
		} else if alpha > 255 {
			alpha = 255
		}
		
		// Approximate opacity effect by adjusting foreground color
		if alpha < 255 {
			currentColor := style.GetForeground()
			if currentColor == nil {
				currentColor = lipgloss.Color("#FFFFFF")
			}
			// This is a simplified opacity effect
			style = style.Foreground(currentColor)
		}
		
	case AnimationSlideIn:
		offset := int(lerp(float64(anim.startOffset), float64(anim.endOffset), easedProgress))
		if offset > 0 {
			style = style.MarginLeft(offset)
		} else if offset < 0 {
			style = style.MarginTop(-offset)
		}
		
	case AnimationSlideOut:
		offset := int(lerp(float64(anim.startOffset), float64(anim.endOffset), easedProgress))
		if offset > 0 {
			style = style.MarginLeft(offset)
		} else if offset < 0 {
			style = style.MarginTop(-offset)
		}
		
	case AnimationBounce:
		// Apply a subtle transform for bounce effect
		bounceOffset := int(easedProgress * 2)
		if bounceOffset > 0 {
			style = style.MarginLeft(bounceOffset)
		}
		
	case AnimationPulse:
		// Apply scaling effect for pulse (simplified as padding)
		scale := easedProgress
		padding := int(scale * 2)
		style = style.Padding(0, padding)
	}
	
	return style.Render(content)
}

// Update handles animation updates
func (am *AnimationManager) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case AnimationTickMsg:
		return am.handleAnimationTick(msg)
	}
	return nil
}

// handleAnimationTick handles animation tick messages
func (am *AnimationManager) handleAnimationTick(msg AnimationTickMsg) tea.Cmd {
	anim, exists := am.animations[msg.AnimationID]
	if !exists || anim.state != AnimationStateRunning {
		return nil
	}
	
	elapsed := time.Since(anim.startTime)
	progress := float64(elapsed) / float64(anim.duration)
	
	if progress >= 1.0 {
		// Animation completed
		anim.progress = 1.0
		anim.state = AnimationStateCompleted
		
		if anim.onComplete != nil {
			anim.onComplete()
		}
		
		return nil
	}
	
	anim.progress = progress
	anim.currentFrame++
	
	// Continue animation
	return am.animationTick(msg.AnimationID)
}

// animationTick creates an animation tick command
func (am *AnimationManager) animationTick(id string) tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(t time.Time) tea.Msg {
		anim := am.animations[id]
		return AnimationTickMsg{
			AnimationID: id,
			Frame:       anim.currentFrame + 1,
			Progress:    anim.progress,
		}
	})
}

// SetOnComplete sets a callback for when the animation completes
func (am *AnimationManager) SetOnComplete(id string, callback func()) {
	if anim, exists := am.animations[id]; exists {
		anim.onComplete = callback
	}
}

// Easing functions

// easeInOutCubic provides smooth acceleration and deceleration
func easeInOutCubic(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - math.Pow(-2*t+2, 3)/2
}

// bounceEasing provides a bounce effect
func bounceEasing(t float64) float64 {
	const n1 = 7.5625
	const d1 = 2.75
	
	if t < 1/d1 {
		return n1 * t * t
	} else if t < 2/d1 {
		t -= 1.5 / d1
		return n1*t*t + 0.75
	} else if t < 2.5/d1 {
		t -= 2.25 / d1
		return n1*t*t + 0.9375
	} else {
		t -= 2.625 / d1
		return n1*t*t + 0.984375
	}
}

// pulseEasing provides a pulse effect
func pulseEasing(t float64) float64 {
	return 0.5 * (1 + math.Sin(2*math.Pi*t-math.Pi/2))
}

// lerp performs linear interpolation
func lerp(start, end, t float64) float64 {
	return start + t*(end-start)
}

// Utility functions for common animation patterns

// CreateViewTransition creates a smooth transition between views
func CreateViewTransition(id string, fromContent, toContent string, duration time.Duration) *Animation {
	am := NewAnimationManager()
	return am.CreateFadeInAnimation(id, duration)
}

// CreateLoadingAnimation creates a loading animation
func CreateLoadingAnimation(id string) *Animation {
	am := NewAnimationManager()
	return am.CreatePulseAnimation(id, 1500*time.Millisecond)
}

// CreateErrorAnimation creates an error highlight animation
func CreateErrorAnimation(id string) *Animation {
	am := NewAnimationManager()
	return am.CreateBounceAnimation(id, 800*time.Millisecond)
}