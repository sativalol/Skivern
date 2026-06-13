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
			Description: "Ask AI a question or give it a prompt.",
		},
	})
}

var AskCmd = &manager.Command{
	Trigger:     "ask",
	Aliases:     []string{"ai", "gpt"},
	Name:        "ask",
	Description: "Generate content using AI",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("ask")
		}

		provs, err := ctx.DB.ListAIProviders()
		if err != nil || len(provs) == 0 {
			return ctx.Reply("[!] No AI providers configured. Please set one up in the TUI Settings first.")
		}

		var sysMsg string
		prompt := strings.Join(ctx.Args, " ")
		temp := 0.7
		maxT := 1000

		pName := "default"
		if len(ctx.Args) >= 3 && strings.ToLower(ctx.Args[0]) == "-prompt" {
			pName = strings.ToLower(ctx.Args[1])
			prompt = strings.Join(ctx.Args[2:], " ")
		}

		if p, err := ctx.DB.GetAIPrompt(pName); err == nil {
			sysMsg = p.SystemMsg
			if p.Temperature > 0 {
				temp = p.Temperature
			}
			if p.MaxTokens > 0 {
				maxT = p.MaxTokens
			}
		} else if pName == "default" {
			sysMsg = "You are a helpful AI assistant."
		} else {
			return ctx.Reply(fmt.Sprintf("[!] Prompt `%s` not found.", pName))
		}

		_ = ctx.Reply("[*] Thinking, please wait...")

		res, err := ai.Generate(ctx.DB, provs[0].ID, ai.GenOpts{
			UserMsg:     prompt,
			SystemMsg:   sysMsg,
			Temperature: temp,
			MaxTokens:   maxT,
		})

		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] failed: %v", err))
		}

		return ctx.ReplyLarge(res.Text, "ai_response.txt")
	},
}
