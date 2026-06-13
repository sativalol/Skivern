package utility

import (
	"fmt"
	"skyvern/internal/ai/tools"
	"skyvern/internal/manager"
)

func init() {
	manager.RegisterHelp("scrape", []manager.HelpPage{
		{
			Command:     "Scrape",
			Syntax:      ".scrape <url>",
			Description: "Scrapes the text and meta tags from any website URL.",
		},
	})
}

var ScrapeCmd = &manager.Command{
	Trigger:     "scrape",
	Name:        "scrape",
	Description: "Scrape content from a website",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("scrape")
		}

		u := ctx.Args[0]
		_ = ctx.Reply("[*] Scraping URL, please wait...")

		res, err := tools.Scrape(u)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Scrape failed: %v", err))
		}

		titleStr := ""
		if res.Title != "" {
			titleStr = fmt.Sprintf("Title: %s\n\n", res.Title)
		}
		
		output := fmt.Sprintf("%s%s", titleStr, res.TextContent)
		return ctx.ReplyLarge(output, "scrape.txt")
	},
}
