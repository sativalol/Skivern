package fun

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("anime", []manager.HelpPage{
		{
			Command:     "Anime Search",
			Syntax:      ".anime <query>",
			Description: "Search MyAnimeList for anime details.",
		},
	})
	manager.RegisterHelp("character", []manager.HelpPage{
		{
			Command:     "Character Search",
			Syntax:      ".character <query>",
			Description: "Search MyAnimeList for character details.",
		},
	})
	manager.RegisterHelp("book", []manager.HelpPage{
		{
			Command:     "Book Search",
			Syntax:      ".book <query>",
			Description: "Search for books using OpenLibrary.",
		},
	})
	manager.RegisterHelp("tvshow", []manager.HelpPage{
		{
			Command:     "TV Show Search",
			Syntax:      ".tvshow <query>",
			Description: "Search for TV shows using TVmaze.",
		},
	})
	manager.RegisterHelp("twitch", []manager.HelpPage{
		{
			Command:     "Twitch Profile",
			Syntax:      ".twitch <username>",
			Description: "Get Twitch channel details.",
		},
	})
	manager.RegisterHelp("youtube", []manager.HelpPage{
		{
			Command:     "YouTube Search",
			Syntax:      ".youtube <query>",
			Description: "Find a YouTube video.",
		},
	})
	manager.RegisterHelp("game", []manager.HelpPage{
		{
			Command:     "Game Search",
			Syntax:      ".game <query>",
			Description: "Get information about a video game.",
		},
	})
}

var Anime = &manager.Command{
	Trigger:     "anime",
	Name:        "anime",
	Description: "Search MyAnimeList for anime details",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("anime")
		}
		query := strings.Join(ctx.Args, " ")
		res, err := http.Get(fmt.Sprintf("https://api.jikan.moe/v4/anime?q=%s&limit=1", url.QueryEscape(query)))
		if err != nil {
			return ctx.Reply("[!] Jikan API offline.")
		}
		defer res.Body.Close()

		var data struct {
			Data []struct {
				Title    string `json:"title"`
				Synopsis string `json:"synopsis"`
				URL      string `json:"url"`
				Type     string `json:"type"`
				Episodes int    `json:"episodes"`
				Score    float64 `json:"score"`
				Images   struct {
					JPG struct {
						ImageURL string `json:"image_url"`
					} `json:"jpg"`
				} `json:"images"`
			} `json:"data"`
		}

		if err := json.NewDecoder(res.Body).Decode(&data); err != nil || len(data.Data) == 0 {
			return ctx.Reply(fmt.Sprintf("[!] No anime found for `%s`.", query))
		}

		a := data.Data[0]
		syn := a.Synopsis
		if len(syn) > 800 {
			syn = syn[:797] + "..."
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       a.Title,
			Description: syn,
		})
		emb.URL = a.URL
		if a.Images.JPG.ImageURL != "" {
			emb.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: a.Images.JPG.ImageURL}
		}
		emb.Fields = []*discordgo.MessageEmbedField{
			{Name: "Type", Value: a.Type, Inline: true},
			{Name: "Episodes", Value: fmt.Sprintf("%d", a.Episodes), Inline: true},
			{Name: "Score", Value: fmt.Sprintf("%.2f", a.Score), Inline: true},
		}
		return ctx.Respond(emb)
	},
}

var Character = &manager.Command{
	Trigger:     "character",
	Name:        "character",
	Description: "Search MyAnimeList characters",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("character")
		}
		query := strings.Join(ctx.Args, " ")
		res, err := http.Get(fmt.Sprintf("https://api.jikan.moe/v4/characters?q=%s&limit=1", url.QueryEscape(query)))
		if err != nil {
			return ctx.Reply("[!] Jikan API offline.")
		}
		defer res.Body.Close()

		var data struct {
			Data []struct {
				Name  string `json:"name"`
				About string `json:"about"`
				URL   string `json:"url"`
				Images struct {
					JPG struct {
						ImageURL string `json:"image_url"`
					} `json:"jpg"`
				} `json:"images"`
			} `json:"data"`
		}

		if err := json.NewDecoder(res.Body).Decode(&data); err != nil || len(data.Data) == 0 {
			return ctx.Reply(fmt.Sprintf("[!] Character `%s` not found.", query))
		}

		c := data.Data[0]
		about := c.About
		if len(about) > 800 {
			about = about[:797] + "..."
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       c.Name,
			Description: about,
		})
		emb.URL = c.URL
		if c.Images.JPG.ImageURL != "" {
			emb.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: c.Images.JPG.ImageURL}
		}
		return ctx.Respond(emb)
	},
}

var Book = &manager.Command{
	Trigger:     "book",
	Name:        "book",
	Description: "Search for books using OpenLibrary",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("book")
		}
		query := strings.Join(ctx.Args, " ")
		res, err := http.Get(fmt.Sprintf("https://openlibrary.org/search.json?q=%s&limit=1", url.QueryEscape(query)))
		if err != nil {
			return ctx.Reply("[!] OpenLibrary API offline.")
		}
		defer res.Body.Close()

		var data struct {
			Docs []struct {
				Title       string   `json:"title"`
				AuthorName  []string `json:"author_name"`
				FirstPub    int      `json:"first_publish_year"`
				Key         string   `json:"key"`
				CoverI      int      `json:"cover_i"`
			} `json:"docs"`
		}

		if err := json.NewDecoder(res.Body).Decode(&data); err != nil || len(data.Docs) == 0 {
			return ctx.Reply(fmt.Sprintf("[!] No books found for `%s`.", query))
		}

		b := data.Docs[0]
		authors := "Unknown"
		if len(b.AuthorName) > 0 {
			authors = strings.Join(b.AuthorName, ", ")
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       b.Title,
			Description: fmt.Sprintf("**Author(s):** %s\n**First Published:** %d", authors, b.FirstPub),
		})
		emb.URL = fmt.Sprintf("https://openlibrary.org%s", b.Key)
		if b.CoverI != 0 {
			emb.Thumbnail = &discordgo.MessageEmbedThumbnail{
				URL: fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-M.jpg", b.CoverI),
			}
		}
		return ctx.Respond(emb)
	},
}

var TVShow = &manager.Command{
	Trigger:     "tvshow",
	Aliases:     []string{"tv", "series"},
	Name:        "tvshow",
	Description: "Search for TV shows using TVmaze",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("tvshow")
		}
		query := strings.Join(ctx.Args, " ")
		res, err := http.Get(fmt.Sprintf("https://api.tvmaze.com/singlesearch/shows?q=%s", url.QueryEscape(query)))
		if err != nil {
			return ctx.Reply("[!] TVmaze API offline.")
		}
		defer res.Body.Close()

		if res.StatusCode != 200 {
			return ctx.Reply(fmt.Sprintf("[!] TV Show `%s` not found.", query))
		}

		var s struct {
			Name    string `json:"name"`
			Summary string `json:"summary"`
			URL     string `json:"url"`
			Status  string `json:"status"`
			Rating  struct {
				Average float64 `json:"average"`
			} `json:"rating"`
			Image struct {
				Medium string `json:"medium"`
			} `json:"image"`
		}

		if err := json.NewDecoder(res.Body).Decode(&s); err != nil {
			return ctx.Reply("[!] Error parsing response.")
		}

		sum := s.Summary
		sum = strings.ReplaceAll(sum, "<p>", "")
		sum = strings.ReplaceAll(sum, "</p>", "")
		sum = strings.ReplaceAll(sum, "<b>", "**")
		sum = strings.ReplaceAll(sum, "</b>", "**")
		sum = strings.ReplaceAll(sum, "<i>", "*")
		sum = strings.ReplaceAll(sum, "</i>", "*")
		if len(sum) > 800 {
			sum = sum[:797] + "..."
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       s.Name,
			Description: sum,
		})
		emb.URL = s.URL
		if s.Image.Medium != "" {
			emb.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: s.Image.Medium}
		}
		emb.Fields = []*discordgo.MessageEmbedField{
			{Name: "Status", Value: s.Status, Inline: true},
			{Name: "Rating", Value: fmt.Sprintf("%.1f", s.Rating.Average), Inline: true},
		}
		return ctx.Respond(emb)
	},
}

var Twitch = &manager.Command{
	Trigger:     "twitch",
	Aliases:     []string{"live"},
	Name:        "twitch",
	Description: "Get Twitch channel details",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("twitch")
		}
		user := ctx.Args[0]
		upres, _ := http.Get(fmt.Sprintf("https://decapi.me/twitch/uptime/%s", user))
		uptime := "Offline"
		if upres != nil {
			defer upres.Body.Close()
			if b, err := io.ReadAll(upres.Body); err == nil {
				uptime = string(b)
			}
		}

		title := "Offline"
		if !strings.Contains(strings.ToLower(uptime), "offline") {
			tres, _ := http.Get(fmt.Sprintf("https://decapi.me/twitch/title/%s", user))
			if tres != nil {
				defer tres.Body.Close()
				if b, err := io.ReadAll(tres.Body); err == nil {
					title = string(b)
				}
			}
		}

		avatar := ""
		avres, _ := http.Get(fmt.Sprintf("https://decapi.me/twitch/avatar/%s", user))
		if avres != nil {
			defer avres.Body.Close()
			if b, err := io.ReadAll(avres.Body); err == nil {
				avatar = string(b)
			}
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       fmt.Sprintf("Twitch: %s", user),
			Description: fmt.Sprintf("**Status:** %s\n**Stream:** %s", uptime, title),
		})
		emb.URL = fmt.Sprintf("https://twitch.tv/%s", user)
		if avatar != "" && !strings.Contains(avatar, "No user") {
			emb.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: avatar}
		}
		return ctx.Respond(emb)
	},
}

var Youtube = &manager.Command{
	Trigger:     "youtube",
	Aliases:     []string{"yt"},
	Name:        "youtube",
	Description: "Find a YouTube video",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("youtube")
		}
		query := strings.Join(ctx.Args, " ")
		req, _ := http.NewRequest("GET", fmt.Sprintf("https://html.duckduckgo.com/html/?q=site:youtube.com+watch+%s", url.QueryEscape(query)), nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return ctx.Reply("[!] Search failed.")
		}
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		htmlStr := string(bodyBytes)

		rxVideo := regexp.MustCompile(`youtube\.com/watch\?v=([a-zA-Z0-9_-]+)`)
		match := rxVideo.FindStringSubmatch(htmlStr)
		if len(match) < 2 {
			return ctx.Reply(fmt.Sprintf("https://www.youtube.com/results?search_query=%s", url.QueryEscape(query)))
		}

		return ctx.Reply(fmt.Sprintf("https://www.youtube.com/watch?v=%s", match[1]))
	},
}

var Game = &manager.Command{
	Trigger:     "game",
	Name:        "game",
	Description: "Search Wikipedia for video game info",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("game")
		}
		q := strings.Join(ctx.Args, " ")
		apiURL := fmt.Sprintf("https://en.wikipedia.org/w/api.php?action=query&list=search&srsearch=%s+video+game&format=json", url.QueryEscape(q))
		res, err := http.Get(apiURL)
		if err != nil {
			return ctx.Reply("[!] Wikipedia search API offline.")
		}
		defer res.Body.Close()

		var searchData struct {
			Query struct {
				Search []struct {
					Title string `json:"title"`
				} `json:"search"`
			} `json:"query"`
		}
		if err := json.NewDecoder(res.Body).Decode(&searchData); err != nil || len(searchData.Query.Search) == 0 {
			return ctx.Reply(fmt.Sprintf("[!] No games found for `%s`.", q))
		}

		title := searchData.Query.Search[0].Title
		summaryURL := fmt.Sprintf("https://en.wikipedia.org/api/rest_v1/page/summary/%s", url.PathEscape(strings.ReplaceAll(title, " ", "_")))
		res2, err := http.Get(summaryURL)
		if err != nil {
			return ctx.Reply("[!] Wikipedia summary API offline.")
		}
		defer res2.Body.Close()

		var pageData struct {
			Title       string `json:"title"`
			Extract     string `json:"extract"`
			ContentURLs struct {
				Desktop struct {
					Page string `json:"page"`
				} `json:"desktop"`
			} `json:"content_urls"`
			Thumbnail struct {
				Source string `json:"source"`
			} `json:"thumbnail"`
		}
		if err := json.NewDecoder(res2.Body).Decode(&pageData); err != nil {
			return ctx.Reply("[!] Failed to decode game details.")
		}

		desc := pageData.Extract
		if len(desc) > 800 {
			desc = desc[:797] + "..."
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       pageData.Title,
			Description: desc,
		})
		emb.URL = pageData.ContentURLs.Desktop.Page
		if pageData.Thumbnail.Source != "" {
			emb.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: pageData.Thumbnail.Source}
		}
		return ctx.Respond(emb)
	},
}

