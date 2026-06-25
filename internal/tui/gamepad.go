//go:build !linux

package tui

import tea "github.com/charmbracelet/bubbletea"

type NativeGamepad struct{}

func NewNativeGamepad() *NativeGamepad { return nil }

func (g *NativeGamepad) Next() tea.Cmd { return nil }
