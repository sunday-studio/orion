package cli

import (
	"orion/agent/internal/ui"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// wizardTheme styles huh prompts with the design tokens.
func wizardTheme() *huh.Theme {
	light, dark := ui.Palettes()
	ad := func(l, d string) lipgloss.AdaptiveColor { return lipgloss.AdaptiveColor{Light: l, Dark: d} }
	t := huh.ThemeBase()
	accent, text, muted, faint := ad(light.Accent, dark.Accent), ad(light.Text, dark.Text), ad(light.Muted, dark.Muted), ad(light.Faint, dark.Faint)
	downL, downD := ui.StatusColors("down")
	t.Focused.Title = t.Focused.Title.Foreground(accent).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(muted)
	// huh's focused field has a thick left border; colouring it makes the
	// artboard's green ┃ focus bar visible without relying on colour alone.
	t.Focused.Base = t.Focused.Base.BorderForeground(accent)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(accent)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(accent)
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(text)
	t.Focused.UnselectedPrefix = t.Focused.UnselectedPrefix.Foreground(faint)
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(accent)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(faint)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(ad(downL, downD))
	t.Focused.FocusedButton = t.Focused.FocusedButton.Background(accent).Foreground(ad("#ffffff", "#111513"))
	t.Blurred.Title = t.Blurred.Title.Foreground(muted)
	return t
}
