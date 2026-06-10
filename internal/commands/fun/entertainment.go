package fun

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("activity", []manager.HelpPage{
		{
			Command:     "Activity Check",
			Syntax:      ".activity [@user]",
			Description: "Shows what a user is currently doing on Discord.",
		},
	})
	manager.RegisterHelp("streaming", []manager.HelpPage{
		{
			Command:     "Streaming Check",
			Syntax:      ".streaming [@user]",
			Description: "Shows streaming status details for a user.",
		},
	})
	manager.RegisterHelp("lyrics", []manager.HelpPage{
		{
			Command:     "Lyrics Search",
			Syntax:      ".lyrics <song name>",
			Description: "Get song lyrics using LRCLIB.",
		},
	})
	manager.RegisterHelp("findsong", []manager.HelpPage{
		{
			Command:     "Find Song",
			Syntax:      ".findsong <song name>",
			Description: "Search Genius keylessly for song info.",
		},
	})
	manager.RegisterHelp("find-id", []manager.HelpPage{
		{
			Command:     "Find Artist ID",
			Syntax:      ".find-id <artist name>",
			Description: "Lookup Genius artist ID keylessly.",
		},
	})
	manager.RegisterHelp("kanye", []manager.HelpPage{
		{
			Command:     "Kanye Quote",
			Syntax:      ".kanye",
			Description: "Get a random Kanye West quote.",
		},
	})
	manager.RegisterHelp("compliment", []manager.HelpPage{
		{
			Command:     "Compliment User",
			Syntax:      ".compliment [@user]",
			Description: "Sends a random compliment to a user.",
		},
	})
	manager.RegisterHelp("fact", []manager.HelpPage{
		{
			Command:     "Fun Fact",
			Syntax:      ".fact",
			Description: "Get a random useless fun fact.",
		},
	})
	manager.RegisterHelp("cat", []manager.HelpPage{
		{
			Command:     "Cat Image",
			Syntax:      ".cat",
			Description: "Fetches a random cat image.",
		},
	})
}

var Activity = &manager.Command{
	Trigger:     "activity",
	Aliases:     []string{"presence", "act"},
	Name:        "activity",
	Description: "Shows what a user is currently doing on Discord",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		target := ctx.AuthorID()
		gid := ctx.GuildID()
		if len(ctx.Args) > 0 {
			t := resolveVouchUser(ctx.Session, gid, ctx.Args[0])
			if t != "" {
				target = t
			}
		}

		p, err := ctx.Session.State.Presence(gid, target)
		if err != nil {
			return ctx.Reply("[*] User has no active presence (Offline/Invisible).")
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**Status:** %s\n\n", strings.Title(string(p.Status))))

		hasActivity := false
		for _, act := range p.Activities {
			hasActivity = true
			if act.Name == "Spotify" {
				sb.WriteString(fmt.Sprintf("🎵 **Spotify:**\n*Song:* %s\n*Artist:* %s\n*Album:* %s\n\n", act.Details, act.State, act.Assets.LargeText))
			} else if act.Type == discordgo.ActivityTypeCustom {
				sb.WriteString(fmt.Sprintf("💭 **Status:** %s\n\n", act.State))
			} else {
				sb.WriteString(fmt.Sprintf("🎮 **Playing:** %s\n", act.Name))
				if act.Details != "" {
					sb.WriteString(fmt.Sprintf("*Details:* %s\n", act.Details))
				}
				if act.State != "" {
					sb.WriteString(fmt.Sprintf("*State:* %s\n", act.State))
				}
				sb.WriteString("\n")
			}
		}

		if !hasActivity {
			sb.WriteString("No active custom status or activities.")
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       fmt.Sprintf("Activity: %s", p.User.Username),
			Description: sb.String(),
		})
		return ctx.Respond(emb)
	},
}

var Streaming = &manager.Command{
	Trigger:     "streaming",
	Aliases:     []string{"stream"},
	Name:        "streaming",
	Description: "Shows what a user is currently streaming",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		target := ctx.AuthorID()
		gid := ctx.GuildID()
		if len(ctx.Args) > 0 {
			t := resolveVouchUser(ctx.Session, gid, ctx.Args[0])
			if t != "" {
				target = t
			}
		}

		p, err := ctx.Session.State.Presence(gid, target)
		if err != nil {
			return ctx.Reply("[*] User is offline.")
		}

		for _, act := range p.Activities {
			if act.Type == discordgo.ActivityTypeStreaming {
				return ctx.Reply(fmt.Sprintf("[+] **%s** is streaming **%s**!\nWatch here: %s", p.User.Username, act.Details, act.URL))
			}
		}
		return ctx.Reply(fmt.Sprintf("[*] **%s** is not currently streaming.", p.User.Username))
	},
}

var Lyrics = &manager.Command{
	Trigger:     "lyrics",
	Name:        "lyrics",
	Description: "Get song lyrics using LRCLIB",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("lyrics")
		}
		query := strings.Join(ctx.Args, " ")
		res, err := http.Get(fmt.Sprintf("https://lrclib.net/api/search?q=%s", url.QueryEscape(query)))
		if err != nil {
			return ctx.Reply("[!] Lyrics service offline.")
		}
		defer res.Body.Close()

		var results []struct {
			TrackName   string `json:"trackName"`
			ArtistName  string `json:"artistName"`
			PlainLyrics string `json:"plainLyrics"`
		}

		if err := json.NewDecoder(res.Body).Decode(&results); err != nil || len(results) == 0 || results[0].PlainLyrics == "" {
			return ctx.Reply(fmt.Sprintf("[!] Could not find lyrics for `%s`.", query))
		}

		lyr := results[0].PlainLyrics
		if len(lyr) > 2000 {
			lyr = lyr[:1997] + "..."
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       fmt.Sprintf("Lyrics: %s - %s", results[0].TrackName, results[0].ArtistName),
			Description: lyr,
		})
		return ctx.Respond(emb)
	},
}

var FindSong = &manager.Command{
	Trigger:     "findsong",
	Aliases:     []string{"song"},
	Name:        "findsong",
	Description: "Search Genius keylessly for song info",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("findsong")
		}
		query := strings.Join(ctx.Args, " ")
		client := &http.Client{}
		req, _ := http.NewRequest("GET", fmt.Sprintf("https://genius.com/api/search/multi?q=%s", url.QueryEscape(query)), nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
		res, err := client.Do(req)
		if err != nil {
			return ctx.Reply("[!] Genius lookup failed.")
		}
		defer res.Body.Close()

		var data struct {
			Response struct {
				Sections []struct {
					Type string `json:"type"`
					Hits []struct {
						Result struct {
							Title       string `json:"title"`
							ArtistNames string `json:"artist_names"`
							URL         string `json:"url"`
							Image       string `json:"header_image_thumbnail_url"`
							ID          int    `json:"id"`
						} `json:"result"`
					} `json:"hits"`
				} `json:"sections"`
			} `json:"response"`
		}

		if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
			return ctx.Reply("[!] Error parsing Genius response.")
		}

		var songHit struct {
			Title       string
			ArtistNames string
			URL         string
			Image       string
			ID          int
		}
		found := false

		for _, sec := range data.Response.Sections {
			if sec.Type == "song" && len(sec.Hits) > 0 {
				r := sec.Hits[0].Result
				songHit.Title = r.Title
				songHit.ArtistNames = r.ArtistNames
				songHit.URL = r.URL
				songHit.Image = r.Image
				songHit.ID = r.ID
				found = true
				break
			}
		}

		if !found {
			return ctx.Reply(fmt.Sprintf("[!] No songs found for `%s`.", query))
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       fmt.Sprintf("Song: %s", songHit.Title),
			Description: fmt.Sprintf("**Artist:** %s\n**Genius ID:** %d\n[Genius Page](%s)", songHit.ArtistNames, songHit.ID, songHit.URL),
		})
		if songHit.Image != "" {
			emb.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: songHit.Image}
		}
		return ctx.Respond(emb)
	},
}

var FindID = &manager.Command{
	Trigger:     "find-id",
	Name:        "find-id",
	Description: "Lookup Genius artist ID keylessly",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("find-id")
		}
		query := strings.Join(ctx.Args, " ")
		client := &http.Client{}
		req, _ := http.NewRequest("GET", fmt.Sprintf("https://genius.com/api/search/multi?q=%s", url.QueryEscape(query)), nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
		res, err := client.Do(req)
		if err != nil {
			return ctx.Reply("[!] Genius lookup failed.")
		}
		defer res.Body.Close()

		var data struct {
			Response struct {
				Sections []struct {
					Type string `json:"type"`
					Hits []struct {
						Result struct {
							Name string `json:"name"`
							ID   int    `json:"id"`
							URL  string `json:"url"`
						} `json:"result"`
					} `json:"hits"`
				} `json:"sections"`
			} `json:"response"`
		}

		if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
			return ctx.Reply("[!] Error parsing Genius response.")
		}

		for _, sec := range data.Response.Sections {
			if sec.Type == "artist" && len(sec.Hits) > 0 {
				r := sec.Hits[0].Result
				return ctx.Reply(fmt.Sprintf("[+] **Artist:** %s\n**Genius ID:** `%d`\nProfile: %s", r.Name, r.ID, r.URL))
			}
		}
		return ctx.Reply(fmt.Sprintf("[!] Artist `%s` not found.", query))
	},
}

var Kanye = &manager.Command{
	Trigger:     "kanye",
	Aliases:     []string{"ye", "yeezy"},
	Name:        "kanye",
	Description: "Get a random Kanye West quote",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		res, err := http.Get("https://api.kanye.rest/")
		if err != nil {
			return ctx.Reply("[!] Ye API offline.")
		}
		defer res.Body.Close()

		var data struct {
			Quote string `json:"quote"`
		}
		if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
			return ctx.Reply("[!] Error parsing quote.")
		}

		return ctx.Reply(fmt.Sprintf("*\"%s\"* - Kanye West", data.Quote))
	},
}

var Compliment = &manager.Command{
	Trigger:     "compliment",
	Name:        "compliment",
	Description: "Sends a random compliment to a user",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		compliments := []string{
			"You are an absolute asset to this server.",
			"Your code compiles cleanly on the first try.",
			"You bring an amazing energy to every conversation.",
			"Your vibes are immaculate.",
			"You have a great sense of style.",
			"Your intelligence is inspiring.",
			"You are a wonderfully kind person.",
		}
		comp := compliments[rand.Intn(len(compliments))]
		target := "<@" + ctx.AuthorID() + ">"
		if len(ctx.Args) > 0 {
			target = ctx.Args[0]
		}
		return ctx.Reply(fmt.Sprintf("%s %s", target, comp))
	},
}

var Fact = &manager.Command{
	Trigger:     "fact",
	Name:        "fact",
	Description: "Get a random useless fun fact",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		res, err := http.Get("https://uselessfacts.jsph.pl/api/v2/facts/random?language=en")
		if err != nil {
			return ctx.Reply("[!] Fact API offline.")
		}
		defer res.Body.Close()

		var data struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
			return ctx.Reply("[!] Error parsing fact.")
		}
		return ctx.Reply(fmt.Sprintf("**Did you know?** %s", data.Text))
	},
}

var Cat = &manager.Command{
	Trigger:     "cat",
	Aliases:     []string{"kitty", "meow"},
	Name:        "cat",
	Description: "Get a random cat image",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		res, err := http.Get("https://api.thecatapi.com/v1/images/search")
		if err != nil {
			return ctx.Reply("[!] Cat API offline.")
		}
		defer res.Body.Close()

		var data []struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(res.Body).Decode(&data); err != nil || len(data) == 0 {
			return ctx.Reply("[!] Error fetching cat image.")
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title: "🐱 Meow!",
		})
		emb.Image = &discordgo.MessageEmbedImage{URL: data[0].URL}
		return ctx.Respond(emb)
	},
}
