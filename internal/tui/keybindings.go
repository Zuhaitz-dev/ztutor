package tui

// Key constants — change a binding here and it propagates everywhere.
const (
	// ── Global ───────────────────────────────────────────────────────────────
	KeyLanguage    = "ctrl+l"    // cycle UI language (any screen)
	KeyQuit        = "ctrl+c"    // hard-terminate the application
	KeyBack        = "q"         // back / exit current screen
	KeyBackAlt     = "esc"       // alternative back
	KeyBackB       = "b"         // back (lesson-list level)
	KeyBackEditor  = "ctrl+q"    // back from an editor screen (exercise, challenge)
	KeySelect      = "enter"     // confirm / open selected item
	KeySelectAlt   = " "         // alternative confirm (space)
	KeyUp          = "up"        // navigate up (arrow key)
	KeyUpVim       = "k"         // navigate up (vim style)
	KeyDown        = "down"      // navigate down (arrow key)
	KeyDownVim     = "j"         // navigate down (vim style)
	KeyLeft        = "left"      // navigate left / Konami
	KeyRight       = "right"     // navigate right / Konami
	KeySection     = "tab"       // advance to next section tab
	KeySectionPrev = "shift+tab" // go back one section tab
	KeySearch      = "/"         // open inline search
	KeyMochi       = "m"         // toggle Mochi companion panel
	KeyScrollTop   = "g"         // scroll to top
	KeyScrollBot   = "G"         // scroll to bottom

	// ── Menu-level actions ────────────────────────────────────────────────────
	KeyAchieve  = "a" // open achievements screen
	KeyRanks    = "l" // open leaderboard screen
	KeySettings = "s" // open settings screen
	KeyCredits  = "c" // open credits screen
	KeyReview   = "r" // jump to next incomplete lesson (review mode)
	KeyAdmin    = "A" // open admin panel

	// ── Exercise / Editor ─────────────────────────────────────────────────────
	KeyRun          = "ctrl+s"    // compile & run / submit
	KeyInteract     = "ctrl+e"    // interactive stdin mode
	KeyAsan         = "ctrl+d"    // ASAN / sanitizer run
	KeyGdb          = "ctrl+g"    // GDB debug session
	KeyAsm          = "ctrl+a"    // toggle assembly view
	KeyInputs       = "ctrl+f"    // cycle input panels (flags → args → stdin → editor)
	KeyStdin        = "ctrl+n"    // focus stdin panel directly
	KeyOutput       = "ctrl+o"    // toggle output-panel focus
	KeyHintEx       = "?"         // show next hint (exercise screen)
	KeyTrivia       = "."         // cycle trivia facts
	KeyRef          = "ctrl+r"    // show references sidebar
	KeyTimer        = "ctrl+t"    // toggle timer display
	KeyHelp         = "f1"        // toggle keybindings overlay
	KeyFullEditor   = "f4"        // fullscreen editor
	KeyFullAssembly = "f5"        // fullscreen assembly
	KeyFullOutput   = "f6"        // fullscreen output
	KeyFileList     = "ctrl+b"    // jump to file list sidebar
	KeyHexView      = "f2"        // toggle hex memory viewer
	KeyStructView   = "f3"        // toggle struct inspector
	KeyAsmAnnotate  = "a"         // toggle annotated assembly (asm mode only)
	KeyOutputGrow   = "ctrl+up"   // make output panel taller (divider moves up)
	KeyOutputShrink = "ctrl+down" // make output panel shorter (divider moves down)
)
