package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Slot describes a single region in a TerminalLayout. Fixed slots render at
// their natural height (measured via lipgloss.Height). Flex slots share
// remaining terminal height proportionally by Weight.
type Slot struct {
	ID      string
	Flex    bool
	Weight  int
	MinH    int
	Visible func() bool
	Render  func(width, allocated int) string
}

// TerminalLayout allocates terminal height across a list of slots. Fixed slots
// are measured first; the remaining height is distributed among flex slots by
// weight. Output is composited into a Canvas so every row is always exactly
// width visible characters wide, preventing stale content from BubbleTea's
// diff renderer regardless of what individual widgets produce.
type TerminalLayout struct {
	slots  []Slot
	width  int
	height int
}

// NewTerminalLayout creates a layout for the given terminal dimensions.
func NewTerminalLayout(w, h int) *TerminalLayout {
	return &TerminalLayout{width: w, height: h}
}

func (l *TerminalLayout) AddFixed(id string, visible func() bool, render func(int) string) {
	l.slots = append(l.slots, Slot{
		ID: id, Flex: false, Visible: visible,
		Render: func(w, _ int) string { return render(w) },
	})
}

func (l *TerminalLayout) AddFlex(id string, weight int, visible func() bool, render func(int, int) string) {
	if weight < 1 {
		weight = 1
	}
	l.slots = append(l.slots, Slot{
		ID: id, Flex: true, Weight: weight, MinH: 3, Visible: visible,
		Render: func(w, h int) string { return render(w, h) },
	})
}

func (l *TerminalLayout) AddFlexMin(id string, weight, minH int, visible func() bool, render func(int, int) string) {
	if weight < 1 {
		weight = 1
	}
	if minH < 1 {
		minH = 1
	}
	l.slots = append(l.slots, Slot{
		ID: id, Flex: true, Weight: weight, MinH: minH, Visible: visible,
		Render: func(w, h int) string { return render(w, h) },
	})
}

type renderedSlot struct {
	content string
	height  int
	order   int
}

// Render measures fixed slots, allocates flex heights, renders all visible
// slots, and composites the results into a Canvas in original slot order.
func (l *TerminalLayout) Render() string {
	var fixedSlots []renderedSlot
	var flexIndices []int
	remaining := l.height

	for i, s := range l.slots {
		if s.Visible != nil && !s.Visible() {
			continue
		}
		if s.Flex {
			flexIndices = append(flexIndices, i)
			continue
		}
		content := s.Render(l.width, 0)
		h := lipgloss.Height(content)
		if h < 0 {
			h = 0
		}
		fixedSlots = append(fixedSlots, renderedSlot{content: content, height: h, order: i})
		remaining -= h
	}

	if remaining < 0 {
		remaining = 0
	}

	// Distribute remaining height among flex slots proportionally by weight.
	flexCount := len(flexIndices)
	allocations := make([]int, flexCount)
	if flexCount > 0 {
		totalWeight := 0
		for _, idx := range flexIndices {
			totalWeight += l.slots[idx].Weight
		}
		if totalWeight == 0 {
			totalWeight = flexCount
		}
		// When remaining is tight, shrink MinH proportionally so no slot
		// forces an overallocation past the canvas.
		effectiveMinH := flexCount
		if remaining/flexCount < 1 {
			effectiveMinH = 1
		} else {
			effectiveMinH = 1 // at least 1 line per flex slot
		}
		allocated := 0
		for i, idx := range flexIndices {
			s := l.slots[idx]
			share := remaining * s.Weight / totalWeight
			future := flexCount - i - 1
			maxShare := remaining - allocated - future
			if maxShare < 1 {
				maxShare = 1
			}
			if share > maxShare {
				share = maxShare
			}
			if share < effectiveMinH {
				share = effectiveMinH
			}
			if share > maxShare {
				share = maxShare
			}
			if i == flexCount-1 {
				share = remaining - allocated
				if share < 1 {
					share = 1
				}
			}
			allocations[i] = share
			allocated += share
		}
	}

	// Render flex slots with their allocated heights.
	var flexSlots []renderedSlot
	for i, idx := range flexIndices {
		s := l.slots[idx]
		h := allocations[i]
		if h < 1 {
			h = 1
		}
		content := s.Render(l.width, h)
		flexSlots = append(flexSlots, renderedSlot{content: content, height: h, order: idx})
	}

	// Composite all slots into the canvas in their original declaration order.
	canvas := NewCanvas(l.width, l.height)
	byOrder := make(map[int]*renderedSlot, len(fixedSlots)+len(flexSlots))
	for i := range fixedSlots {
		byOrder[fixedSlots[i].order] = &fixedSlots[i]
	}
	for i := range flexSlots {
		byOrder[flexSlots[i].order] = &flexSlots[i]
	}

	cursor := 0
	for i := range l.slots {
		r, ok := byOrder[i]
		if !ok {
			continue
		}
		if r.height == 0 {
			continue
		}
		canvas.DrawAt(cursor, r.content, r.height)
		cursor += r.height
	}

	return canvas.String()
}
