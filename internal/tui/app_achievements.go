package tui

import "fmt"

// grantAchievements evaluates a list of event strings and grants the
// corresponding achievements, appending notification strings for newly unlocked
// ones to a.pendingNotifications.
func (a *App) grantAchievements(events []string) {
	// Load already-earned achievements once to avoid duplicating notifications.
	earned, _ := a.db.GetAchievements(a.username)
	alreadyEarned := make(map[string]bool, len(earned))
	for _, id := range earned {
		alreadyEarned[id] = true
	}

	tryGrant := func(id string) {
		if alreadyEarned[id] {
			return
		}
		if err := a.db.GrantAchievement(a.username, id); err != nil {
			return
		}
		alreadyEarned[id] = true // prevent double-notification in one batch
		if ach := achievementByID(id); ach != nil {
			notif := fmt.Sprintf("%s %s — %s", ach.Icon, ach.Name, ach.Desc)
			a.pendingNotifications = append(a.pendingNotifications, notif)
		}
	}

	for _, event := range events {
		switch event {
		case "compile":
			tryGrant("first_compile")

		case "pass":
			tryGrant("first_pass")

		case "pass_1attempt":
			tryGrant("one_shot")

		case "pass_5attempts":
			tryGrant("comeback")

		case "pass_3star":
			tryGrant("perfect")
			// Check five_perfect threshold.
			count, err := a.db.CountLessonsWithMinStars(a.username, 3)
			if err == nil && count >= 5 {
				tryGrant("five_perfect")
			}

		case "pass_nowarnings":
			tryGrant("clean_coder")

		case "gdb":
			tryGrant("debugger")

		case "asm":
			tryGrant("disassembler")

		case "interactive":
			tryGrant("interactive")

		case "asan":
			tryGrant("sanitized")

		case "segfault_king":
			tryGrant("segfault_king")

		case "into_the_loop":
			tryGrant("into_the_loop")

		case "beer":
			tryGrant("beer")

		case "konami":
			tryGrant("konami")
		}
	}
}
