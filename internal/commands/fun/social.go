package fun

import (
	"encoding/json"
	"fmt"
	"net/http"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("github", []manager.HelpPage{
		{
			Command:     "GitHub Search",
			Syntax:      ".github <username> OR .github <owner>/<repo>",
			Description: "Look up a GitHub user or repository.",
		},
	})
	manager.RegisterHelp("cashapp", []manager.HelpPage{
		{
			Command:     "Cash App Lookup",
			Syntax:      ".cashapp <cashtag>",
			Description: "Verify if a Cash App $Cashtag exists.",
		},
	})
	manager.RegisterHelp("tiktok", []manager.HelpPage{
		{
			Command:     "TikTok Profile",
			Syntax:      ".tiktok <username>",
			Description: "Get TikTok profile link.",
		},
	})
	manager.RegisterHelp("twitter", []manager.HelpPage{
		{
			Command:     "Twitter Profile",
			Syntax:      ".twitter <username>",
			Description: "Get Twitter profile link.",
		},
	})
	manager.RegisterHelp("spotify", []manager.HelpPage{
		{
			Command:     "Spotify Profile",
			Syntax:      ".spotify <username>",
			Description: "Get Spotify profile link or search presence.",
		},
	})
}

var Github = &manager.Command{
	Trigger:     "github",
	Aliases:     []string{"gh", "repo"},
	Name:        "github",
	Description: "Lookup GitHub user or repository",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("github")
		}
		arg := ctx.Args[0]
		client := &http.Client{}

		if strings.Contains(arg, "/") {
			req, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s", arg), nil)
			req.Header.Set("User-Agent", "Skyvern-Bot")
			resp, err := client.Do(req)
			if err != nil || resp.StatusCode != 200 {
				return ctx.Reply("[!] Repository not found or API rate limit hit.")
			}
			defer resp.Body.Close()

			var r struct {
				FullName  string `json:"full_name"`
				Desc      string `json:"description"`
				Stars     int    `json:"stargazers_count"`
				Forks     int    `json:"forks_count"`
				Issues    int    `json:"open_issues_count"`
				HTMLURL   string `json:"html_url"`
				Owner     struct {
					Avatar string `json:"avatar_url"`
				} `json:"owner"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&r)

			emb := config.Build(ctx.Cfg, config.EmbedOpt{
				Title:       r.FullName,
				Description: r.Desc,
			})
			emb.URL = r.HTMLURL
			emb.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: r.Owner.Avatar}
			emb.Fields = []*discordgo.MessageEmbedField{
				{Name: "Stars", Value: fmt.Sprintf("%d ⭐", r.Stars), Inline: true},
				{Name: "Forks", Value: fmt.Sprintf("%d 🍴", r.Forks), Inline: true},
				{Name: "Open Issues", Value: fmt.Sprintf("%d ⚠️", r.Issues), Inline: true},
			}
			return ctx.Respond(emb)
		}

		req, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/users/%s", arg), nil)
		req.Header.Set("User-Agent", "Skyvern-Bot")
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != 200 {
			return ctx.Reply("[!] User not found or API rate limit hit.")
		}
		defer resp.Body.Close()

		var u struct {
			Login   string `json:"login"`
			Name    string `json:"name"`
			Bio     string `json:"bio"`
			Repos   int    `json:"public_repos"`
			Follows int    `json:"followers"`
			Avatar  string `json:"avatar_url"`
			HTMLURL string `json:"html_url"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&u)

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       fmt.Sprintf("%s (%s)", u.Name, u.Login),
			Description: u.Bio,
		})
		emb.URL = u.HTMLURL
		emb.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: u.Avatar}
		emb.Fields = []*discordgo.MessageEmbedField{
			{Name: "Public Repos", Value: fmt.Sprintf("%d", u.Repos), Inline: true},
			{Name: "Followers", Value: fmt.Sprintf("%d", u.Follows), Inline: true},
		}
		return ctx.Respond(emb)
	},
}

var Cashapp = &manager.Command{
	Trigger:     "cashapp",
	Aliases:     []string{"cash", "ca"},
	Name:        "cashapp",
	Description: "Verify if a Cash App $Cashtag exists",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("cashapp")
		}
		tag := strings.TrimPrefix(ctx.Args[0], "$")
		client := &http.Client{}
		req, _ := http.NewRequest("GET", fmt.Sprintf("https://cash.app/qr/$%s", tag), nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
		resp, err := client.Do(req)
		if err != nil {
			return ctx.Reply("[!] Cash App lookup failed.")
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			return ctx.Reply(fmt.Sprintf("[+] **$%s** is a valid Cash App account.\nProfile: https://cash.app/$%s", tag, tag))
		}
		return ctx.Reply(fmt.Sprintf("[!] **$%s** does not exist or is inactive.", tag))
	},
}

var Tiktok = &manager.Command{
	Trigger:     "tiktok",
	Aliases:     []string{"tt"},
	Name:        "tiktok",
	Description: "Get TikTok profile link",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("tiktok")
		}
		username := strings.TrimPrefix(ctx.Args[0], "@")
		return ctx.Reply(fmt.Sprintf("TikTok: https://www.tiktok.com/@%s", username))
	},
}

var Twitter = &manager.Command{
	Trigger:     "twitter",
	Aliases:     []string{"x", "tw"},
	Name:        "twitter",
	Description: "Get Twitter profile link",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("twitter")
		}
		username := strings.TrimPrefix(ctx.Args[0], "@")
		return ctx.Reply(fmt.Sprintf("X/Twitter: https://x.com/%s", username))
	},
}

var Spotify = &manager.Command{
	Trigger:     "spotify",
	Aliases:     []string{"sp"},
	Name:        "spotify",
	Description: "Get Spotify profile link or search presence",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("spotify")
		}
		user := ctx.Args[0]
		return ctx.Reply(fmt.Sprintf("Spotify: https://open.spotify.com/user/%s", user))
	},
}
