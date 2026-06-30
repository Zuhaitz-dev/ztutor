package tui

import (
	"strings"

	"ztutor/internal/i18n"
)

// Key constants: change a binding here and it propagates everywhere.
const (
	// Global
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

	// Menu-level actions
	KeyAchieve  = "a" // open achievements screen
	KeyRanks    = "l" // open leaderboard screen
	KeySettings = "s" // open settings screen
	KeyCredits  = "c" // open credits screen
	KeyReview   = "r" // jump to next incomplete lesson (review mode)
	KeyAdmin    = "A" // open admin panel

	// Exercise / Editor
	KeyRun          = "ctrl+s"    // compile & run / submit
	KeyInteract     = "ctrl+e"    // interactive stdin mode
	KeyAsan         = "ctrl+d"    // ASAN / sanitizer run
	KeyGdb          = "ctrl+g"    // GDB debug session
	KeyAsm          = "ctrl+a"    // toggle assembly view
	KeyInputs       = "ctrl+f"    // cycle input panels (flags → args → stdin → editor)
	KeyStdin        = "ctrl+n"    // focus stdin panel directly
	KeyOutput       = "ctrl+o"    // toggle output-panel focus
	KeyHintEx       = "F7"        // show next hint (exercise screen)
	KeyTrivia       = "F9"        // cycle trivia facts
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

type KeyActionID string

const (
	ActionNavigate       KeyActionID = "navigate"
	ActionSelect         KeyActionID = "select"
	ActionBack           KeyActionID = "back"
	ActionMenuBack       KeyActionID = "menu_back"
	ActionEscBack        KeyActionID = "esc_back"
	ActionExerciseBack   KeyActionID = "exercise_back"
	ActionQuit           KeyActionID = "quit"
	ActionSearch         KeyActionID = "search"
	ActionLanguage       KeyActionID = "language"
	ActionSection        KeyActionID = "section"
	ActionReview         KeyActionID = "review"
	ActionAchievements   KeyActionID = "achievements"
	ActionLeaderboard    KeyActionID = "leaderboard"
	ActionSettings       KeyActionID = "settings"
	ActionCredits        KeyActionID = "credits"
	ActionMochi          KeyActionID = "mochi"
	ActionAdmin          KeyActionID = "admin"
	ActionSkip           KeyActionID = "skip"
	ActionChoose         KeyActionID = "choose"
	ActionConfirm        KeyActionID = "confirm"
	ActionSubmitAnswer   KeyActionID = "submit_answer"
	ActionNextQuestion   KeyActionID = "next_question"
	ActionScroll         KeyActionID = "scroll"
	ActionTopEnd         KeyActionID = "top_end"
	ActionRevealAnswer   KeyActionID = "reveal_answer"
	ActionHideAnswer     KeyActionID = "hide_answer"
	ActionExercise       KeyActionID = "exercise"
	ActionRun            KeyActionID = "run"
	ActionSubmit         KeyActionID = "submit"
	ActionChallengeBack  KeyActionID = "challenge_back"
	ActionInteractive    KeyActionID = "interactive"
	ActionASAN           KeyActionID = "asan"
	ActionGDB            KeyActionID = "gdb"
	ActionAssembly       KeyActionID = "assembly"
	ActionInputs         KeyActionID = "inputs"
	ActionOutput         KeyActionID = "output"
	ActionHint           KeyActionID = "hint"
	ActionHintStart      KeyActionID = "hint_start"
	ActionTrivia         KeyActionID = "trivia"
	ActionFileList       KeyActionID = "file_list"
	ActionFileListMove   KeyActionID = "file_list_move"
	ActionFileListSelect KeyActionID = "file_list_select"
	ActionSend           KeyActionID = "send"
	ActionKill           KeyActionID = "kill"
	ActionAsmResize      KeyActionID = "asm_resize"
	ActionAsmAnnotate    KeyActionID = "asm_annotate"
	ActionAsmHide        KeyActionID = "asm_hide"
	ActionOutputResize   KeyActionID = "output_resize"
	ActionNext           KeyActionID = "next"
	ActionReferences     KeyActionID = "references"
	ActionTimer          KeyActionID = "timer"
	ActionHelp           KeyActionID = "help"
	ActionFullEditor     KeyActionID = "full_editor"
	ActionFullAssembly   KeyActionID = "full_assembly"
	ActionFullOutput     KeyActionID = "full_output"
	ActionHexView        KeyActionID = "hex_view"
	ActionStructView     KeyActionID = "struct_view"
	ActionToggleSettings KeyActionID = "toggle_settings"
)

type KeyAction struct {
	ID       KeyActionID
	Keys     []string
	LabelKey string
}

type HelpAction struct {
	ID   KeyActionID
	Args []interface{}
}

func HA(id KeyActionID, args ...interface{}) HelpAction {
	return HelpAction{ID: id, Args: args}
}

var keyActionRegistry = map[KeyActionID]KeyAction{
	ActionNavigate:       {ID: ActionNavigate, Keys: []string{KeyDownVim, KeyUpVim}, LabelKey: "help.navigate"},
	ActionSelect:         {ID: ActionSelect, Keys: []string{KeySelect}, LabelKey: "help.select"},
	ActionBack:           {ID: ActionBack, Keys: []string{KeyBack}, LabelKey: "help.q_back"},
	ActionMenuBack:       {ID: ActionMenuBack, Keys: []string{KeyBackB}, LabelKey: "help.back"},
	ActionEscBack:        {ID: ActionEscBack, Keys: []string{KeyBackAlt}, LabelKey: "help.esc_back"},
	ActionExerciseBack:   {ID: ActionExerciseBack, Keys: []string{KeyBackEditor}, LabelKey: "exercise.help.back"},
	ActionQuit:           {ID: ActionQuit, Keys: []string{KeyBack}, LabelKey: "help.quit"},
	ActionSearch:         {ID: ActionSearch, Keys: []string{KeySearch}, LabelKey: "help.search"},
	ActionLanguage:       {ID: ActionLanguage, Keys: []string{KeyLanguage}, LabelKey: "help.language"},
	ActionSection:        {ID: ActionSection, Keys: []string{KeySection}, LabelKey: "help.section"},
	ActionReview:         {ID: ActionReview, Keys: []string{KeyReview}, LabelKey: "help.review"},
	ActionAchievements:   {ID: ActionAchievements, Keys: []string{KeyAchieve}, LabelKey: "help.achieve"},
	ActionLeaderboard:    {ID: ActionLeaderboard, Keys: []string{KeyRanks}, LabelKey: "help.ranks"},
	ActionSettings:       {ID: ActionSettings, Keys: []string{KeySettings}, LabelKey: "help.settings"},
	ActionCredits:        {ID: ActionCredits, Keys: []string{KeyCredits}, LabelKey: "help.credits"},
	ActionMochi:          {ID: ActionMochi, Keys: []string{KeyMochi}, LabelKey: "help.mochi"},
	ActionAdmin:          {ID: ActionAdmin, Keys: []string{KeyAdmin}, LabelKey: "help.admin"},
	ActionSkip:           {ID: ActionSkip, Keys: []string{KeyBack}, LabelKey: "help.q_skip"},
	ActionChoose:         {ID: ActionChoose, Keys: []string{KeyDownVim, KeyUpVim}, LabelKey: "help.choose"},
	ActionConfirm:        {ID: ActionConfirm, Keys: []string{KeySelect}, LabelKey: "help.confirm"},
	ActionSubmitAnswer:   {ID: ActionSubmitAnswer, Keys: []string{KeySelect}, LabelKey: "quiz.help.submit"},
	ActionNextQuestion:   {ID: ActionNextQuestion, Keys: []string{KeySelect}, LabelKey: "quiz.help.next"},
	ActionScroll:         {ID: ActionScroll, Keys: []string{KeyDownVim, KeyUpVim}, LabelKey: "lesson.help.scroll"},
	ActionTopEnd:         {ID: ActionTopEnd, Keys: []string{KeyScrollTop, KeyScrollBot}, LabelKey: "lesson.help.top_end"},
	ActionRevealAnswer:   {ID: ActionRevealAnswer, Keys: []string{KeyAchieve}, LabelKey: "lesson.help.reveal_answer"},
	ActionHideAnswer:     {ID: ActionHideAnswer, Keys: []string{KeyAchieve}, LabelKey: "lesson.help.hide_answer"},
	ActionExercise:       {ID: ActionExercise, Keys: []string{"e"}, LabelKey: "lesson.help.exercise"},
	ActionRun:            {ID: ActionRun, Keys: []string{KeyRun}, LabelKey: "exercise.help.run"},
	ActionSubmit:         {ID: ActionSubmit, Keys: []string{KeyRun}, LabelKey: "challenge.help.submit"},
	ActionChallengeBack:  {ID: ActionChallengeBack, Keys: []string{KeySelect, KeyBack}, LabelKey: "challenge.help.back"},
	ActionInteractive:    {ID: ActionInteractive, Keys: []string{KeyInteract}, LabelKey: "exercise.help.interactive"},
	ActionASAN:           {ID: ActionASAN, Keys: []string{KeyAsan}, LabelKey: "exercise.help.asan"},
	ActionGDB:            {ID: ActionGDB, Keys: []string{KeyGdb}, LabelKey: "exercise.help.gdb"},
	ActionAssembly:       {ID: ActionAssembly, Keys: []string{KeyAsm}, LabelKey: "exercise.help.asm"},
	ActionInputs:         {ID: ActionInputs, Keys: []string{KeyInputs}, LabelKey: "exercise.help.inputs"},
	ActionOutput:         {ID: ActionOutput, Keys: []string{KeyOutput}, LabelKey: "exercise.help.output"},
	ActionHint:           {ID: ActionHint, Keys: []string{KeyHintEx}, LabelKey: "exercise.help.hint"},
	ActionHintStart:      {ID: ActionHintStart, Keys: []string{KeyHintEx}, LabelKey: "exercise.help.hint_start"},
	ActionTrivia:         {ID: ActionTrivia, Keys: []string{KeyTrivia}, LabelKey: "exercise.help.trivia"},
	ActionFileList:       {ID: ActionFileList, Keys: []string{KeyFileList}, LabelKey: "exercise.help.filelist"},
	ActionFileListMove:   {ID: ActionFileListMove, Keys: []string{KeyDownVim, KeyUpVim}, LabelKey: "exercise.help.filelist.move"},
	ActionFileListSelect: {ID: ActionFileListSelect, Keys: []string{KeySelect}, LabelKey: "exercise.help.filelist.select"},
	ActionSend:           {ID: ActionSend, Keys: []string{KeySelect}, LabelKey: "exercise.help.send"},
	ActionKill:           {ID: ActionKill, Keys: []string{KeyQuit}, LabelKey: "exercise.help.kill"},
	ActionAsmResize:      {ID: ActionAsmResize, Keys: []string{KeyOutputShrink, KeyOutputGrow}, LabelKey: "exercise.help.asm_resize"},
	ActionAsmAnnotate:    {ID: ActionAsmAnnotate, Keys: []string{KeyAsmAnnotate}, LabelKey: "exercise.help.asm_annotate"},
	ActionAsmHide:        {ID: ActionAsmHide, Keys: []string{KeyAsm}, LabelKey: "exercise.help.asm_hide"},
	ActionOutputResize:   {ID: ActionOutputResize, Keys: []string{KeyOutputShrink, KeyOutputGrow}, LabelKey: "exercise.help.output_resize"},
	ActionNext:           {ID: ActionNext, Keys: []string{KeySelect}, LabelKey: "exercise.help.next"},
	ActionReferences:     {ID: ActionReferences, Keys: []string{KeyRef}, LabelKey: "keybindings.references"},
	ActionTimer:          {ID: ActionTimer, Keys: []string{KeyTimer}, LabelKey: "keybindings.timer"},
	ActionHelp:           {ID: ActionHelp, Keys: []string{KeyHelp}, LabelKey: "keybindings.open_help"},
	ActionFullEditor:     {ID: ActionFullEditor, Keys: []string{KeyFullEditor}, LabelKey: "keybindings.full_editor"},
	ActionFullAssembly:   {ID: ActionFullAssembly, Keys: []string{KeyFullAssembly}, LabelKey: "keybindings.full_assembly"},
	ActionFullOutput:     {ID: ActionFullOutput, Keys: []string{KeyFullOutput}, LabelKey: "keybindings.full_output"},
	ActionHexView:        {ID: ActionHexView, Keys: []string{KeyHexView}, LabelKey: "keybindings.hex_view"},
	ActionStructView:     {ID: ActionStructView, Keys: []string{KeyStructView}, LabelKey: "keybindings.struct_view"},
	ActionToggleSettings: {ID: ActionToggleSettings, Keys: []string{KeySelect}, LabelKey: "settings.help.toggle"},
}

func LookupKeyAction(id KeyActionID) (KeyAction, bool) {
	action, ok := keyActionRegistry[id]
	return action, ok
}

func prettyKey(k string) string {
	parts := strings.Split(k, "+")
	for i, p := range parts {
		switch p {
		case "ctrl":
			parts[i] = "Ctrl"
		case "alt":
			parts[i] = "Alt"
		case "shift":
			parts[i] = "Shift"
		case "enter":
			parts[i] = "Enter"
		case "tab":
			parts[i] = "Tab"
		case "esc":
			parts[i] = "Esc"
		case " ":
			parts[i] = "Space"
		case "space":
			parts[i] = "Space"
		case "up":
			parts[i] = "Up"
		case "down":
			parts[i] = "Down"
		case "left":
			parts[i] = "Left"
		case "right":
			parts[i] = "Right"
		default:
			if i > 0 && len(p) == 1 {
				parts[i] = strings.ToUpper(p)
			} else if len(p) >= 2 && p[0] == 'f' {
				rest := p[1:]
				ok := len(rest) > 0
				for _, c := range rest {
					if c < '0' || c > '9' {
						ok = false
						break
					}
				}
				if ok {
					parts[i] = "F" + rest
				}
			}
		}
	}
	return strings.Join(parts, "+")
}

func keyDisplay(keys []string) string {
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if key != "" {
			parts = append(parts, prettyKey(key))
		}
	}
	return strings.Join(parts, "/")
}

func actionLabel(loc *i18n.Locale, action KeyAction, args ...interface{}) string {
	if loc == nil {
		loc = i18n.New("en")
	}
	label := loc.T(action.LabelKey, args...)
	if label == action.LabelKey {
		return string(action.ID)
	}
	display := keyDisplay(action.Keys)
	if display != "" {
		for _, prefix := range []string{display, strings.ToLower(display), strings.ToUpper(display)} {
			if strings.HasPrefix(label, prefix+" ") {
				return strings.TrimSpace(strings.TrimPrefix(label, prefix))
			}
		}
	}
	if idx := strings.Index(label, " "); idx > 0 {
		first := label[:idx]
		if strings.ContainsAny(first, "^/[]?+GgjkqbeA") || strings.EqualFold(first, "enter") || strings.EqualFold(first, "esc") {
			return strings.TrimSpace(label[idx+1:])
		}
	}
	return label
}
