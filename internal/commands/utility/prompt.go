package utility

import (
	"fmt"
	"skyvern/internal/ai"
	"skyvern/internal/manager"
	"strings"
)

func init() {
	manager.RegisterHelp("prompt", []manager.HelpPage{
		{
			Command:     "Prompt Set",
			Syntax:      ".prompt set <name> <systemMsg>",
			Description: "Set a custom named system prompt in prompts.json.",
		},
		{
			Command:     "Prompt Default",
			Syntax:      ".prompt default <systemMsg>",
			Description: "Set the default system prompt for the .ask command.",
		},
		{
			Command:     "Prompt View",
			Syntax:      ".prompt view <name>",
			Description: "View the system message of a prompt.",
		},
		{
			Command:     "Prompt List",
			Syntax:      ".prompt list",
			Description: "List all saved system prompts.",
		},
		{
			Command:     "Prompt Remove",
			Syntax:      ".prompt remove <name>",
			Description: "Delete a system prompt.",
		},
	})
}

var PromptCmd = &manager.Command{
	Trigger:     "prompt",
	Name:        "prompt",
	Description: "Manage system prompts for AI generation",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("prompt")
		}

		m, err := ai.LoadPrompts()
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to load prompts config: %v", err))
		}

		sub := strings.ToLower(ctx.Args[0])
		switch sub {
		case "set":
			if len(ctx.Args) < 3 {
				return ctx.Reply("[!] Syntax: .prompt set <name> <systemMsg>")
			}
			name := strings.ToLower(ctx.Args[1])
			sysMsg := strings.Join(ctx.Args[2:], " ")
			m[name] = sysMsg
			if err := ai.SavePrompts(m); err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to save prompt: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Prompt `%s` saved to JSON.", name))

		case "default":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Syntax: .prompt default <systemMsg>")
			}
			sysMsg := strings.Join(ctx.Args[1:], " ")
			m["default"] = sysMsg
			if err := ai.SavePrompts(m); err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to set default prompt: %v", err))
			}
			return ctx.Reply("[+] Default prompt set successfully in JSON.")

		case "view":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Syntax: .prompt view <name>")
			}
			name := strings.ToLower(ctx.Args[1])
			sysMsg, ok := m[name]
			if !ok {
				return ctx.Reply(fmt.Sprintf("[!] Prompt `%s` not found.", name))
			}
			return ctx.ReplyLarge(fmt.Sprintf("Prompt: %s\n\n%s", name, sysMsg), "prompt_view.txt")

		case "list":
			if len(m) == 0 {
				return ctx.Reply("[*] No custom prompts saved.")
			}
			var sb strings.Builder
			sb.WriteString("Saved System Prompts in JSON:\n\n")
			for name, sysMsg := range m {
				sb.WriteString(fmt.Sprintf("- `%s` (%d chars)\n", name, len(sysMsg)))
			}
			return ctx.Reply(sb.String())

		case "remove", "delete":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Syntax: .prompt remove <name>")
			}
			name := strings.ToLower(ctx.Args[1])
			if _, ok := m[name]; !ok {
				return ctx.Reply(fmt.Sprintf("[!] Prompt `%s` not found.", name))
			}
			delete(m, name)
			if err := ai.SavePrompts(m); err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to delete prompt: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Prompt `%s` removed from JSON.", name))

		default:
			return ctx.SendHelp("prompt")
		}
	},
}
