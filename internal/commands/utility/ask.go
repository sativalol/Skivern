package utility

import (
	"fmt"
	"skyvern/internal/ai"
	"skyvern/internal/manager"
	"strings"
	"time"
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

		prompt := strings.Join(ctx.Args, " ")

		pCfg, err := ai.LoadPrompts()
		sysMsg := pCfg.SystemPrompt

		// Resolve template placeholders
		sysMsg = strings.ReplaceAll(sysMsg, "${currentDate}", time.Now().Format("Monday, January 2, 2006 3:04 PM MST"))
		sysMsg = strings.ReplaceAll(sysMsg, "${userRecognition}", fmt.Sprintf("User: %s (ID: %s)", ctx.AuthorTag(), ctx.AuthorID()))
		sysMsg = strings.ReplaceAll(sysMsg, "${channelContext}", fmt.Sprintf("Channel: <#%s>", ctx.ChanID()))
		sysMsg = strings.ReplaceAll(sysMsg, "${searchInstructions}", "")

		_ = ctx.Reply("[*] Thinking, please wait...")

		res, err := ai.Generate(ctx.DB, provs[0].ID, ai.GenOpts{
			UserMsg:     prompt,
			SystemMsg:   sysMsg,
			Temperature: 0.7,
			MaxTokens:   1200,
		})

		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] failed: %v", err))
		}

		return ctx.ReplyLarge(res.Text, "ai_response.txt")
	},
}
