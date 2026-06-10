package utility

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("screenshot", []manager.HelpPage{
		{
			Command:     "Screenshot",
			Syntax:      ".screenshot <url>",
			Description: "Take a high-quality screenshot of a website.",
		},
	})
}

var Screenshot = &manager.Command{
	Trigger:     "screenshot",
	Aliases:     []string{"ss", "capture"},
	Name:        "screenshot",
	Description: "Take a screenshot of a website",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("screenshot")
		}

		targetURL := ctx.Args[0]
		if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
			targetURL = "https://" + targetURL
		}

		_, err := url.ParseRequestURI(targetURL)
		if err != nil {
			return ctx.Reply("[!] Invalid website URL.")
		}

		_ = ctx.Reply("[*] Capturing screenshot, please wait...")

		apiURL := fmt.Sprintf("https://api.microlink.io?url=%s&screenshot=true&embed=screenshot.url", url.QueryEscape(targetURL))
		resp, err := http.Get(apiURL)
		if err != nil {
			return ctx.Reply("[!] Failed to connect to screenshot service.")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return ctx.Reply("[!] Screenshot service returned an error. Make sure the website exists and is reachable.")
		}

		imgData, err := io.ReadAll(resp.Body)
		if err != nil || len(imgData) == 0 {
			return ctx.Reply("[!] Failed to process screenshot data.")
		}

		fileReader := bytes.NewReader(imgData)
		_, err = ctx.Session.ChannelMessageSendComplex(ctx.ChanID(), &discordgo.MessageSend{
			Content: fmt.Sprintf("[+] Capture of %s:", targetURL),
			Files: []*discordgo.File{
				{
					Name:        "screenshot.png",
					ContentType: "image/png",
					Reader:      fileReader,
				},
			},
		})
		return err
	},
}
