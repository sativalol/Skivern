package utility

import (
	"fmt"
	"skyvern/internal/ai"
	"skyvern/internal/manager"
	"strings"
)

func init() {
	manager.RegisterHelp("ask", []manager.HelpPage{
		{
			Command:     "Ask",
			Syntax:      ".ask <prompt>",
			Description: "Ask configured AI models a question or give them a prompt.",
		},
	})
}

var AskCmd = &manager.Command{
	Trigger:     "ask",
	Aliases:     []string{"ai", "gpt"},
	Name:        "ask",
	Description: "Generate content using configured AI models",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("ask")
		}

		provs, err := ctx.DB.ListAIProviders()
		if err != nil || len(provs) == 0 {
			return ctx.Reply("[!] No AI providers configured. Please set one up in the TUI Settings first.")
		}

		prompt := strings.Join(ctx.Args, " ")
		_ = ctx.Reply("[*] Contacting AI provider, please wait...")

		res, err := ai.Generate(ctx.DB, provs[0].ID, ai.GenOpts{
			UserMsg: prompt,
		})
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] AI generation failed: %v", err))
		}

		return ctx.ReplyLarge(res.Text, "ai_response.txt")
	},
}
