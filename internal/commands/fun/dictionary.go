package fun

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("define", []manager.HelpPage{
		{
			Command:     "Define",
			Syntax:      ".define <word>",
			Description: "Look up a word in the dictionary.",
		},
	})
	manager.RegisterHelp("urban", []manager.HelpPage{
		{
			Command:     "Urban Dictionary",
			Syntax:      ".urban <word>",
			Description: "Look up a word on Urban Dictionary.",
		},
	})
}

var Define = &manager.Command{
	Trigger:     "define",
	Name:        "define",
	Description: "Lookup a word in the dictionary",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("define")
		}
		word := ctx.Args[0]
		res, err := http.Get(fmt.Sprintf("https://api.dictionaryapi.dev/api/v2/entries/en/%s", url.QueryEscape(word)))
		if err != nil {
			return ctx.Reply("[!] Dictionary API offline.")
		}
		defer res.Body.Close()

		if res.StatusCode != 200 {
			return ctx.Reply(fmt.Sprintf("[!] Word `%s` not found.", word))
		}

		var data []struct {
			Word      string `json:"word"`
			Meanings []struct {
				PartOfSpeech string `json:"partOfSpeech"`
				Definitions  []struct {
					Definition string `json:"definition"`
					Example    string `json:"example"`
				} `json:"definitions"`
			} `json:"meanings"`
		}

		if err := json.NewDecoder(res.Body).Decode(&data); err != nil || len(data) == 0 {
			return ctx.Reply("[!] Error decoding dictionary response.")
		}

		w := data[0]
		var sb strings.Builder
		count := 0
		for _, m := range w.Meanings {
			for _, d := range m.Definitions {
				count++
				sb.WriteString(fmt.Sprintf("**%d. [%s]** %s\n", count, m.PartOfSpeech, d.Definition))
				if d.Example != "" {
					sb.WriteString(fmt.Sprintf("*Example:* %s\n", d.Example))
				}
				sb.WriteString("\n")
				if count >= 3 {
					break
				}
			}
			if count >= 3 {
				break
			}
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       fmt.Sprintf("Definition: %s", strings.Title(w.Word)),
			Description: sb.String(),
		})
		return ctx.Respond(emb)
	},
}

var Urban = &manager.Command{
	Trigger:     "urban",
	Name:        "urban",
	Description: "Lookup a word on Urban Dictionary",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("urban")
		}
		query := strings.Join(ctx.Args, " ")
		res, err := http.Get(fmt.Sprintf("https://api.urbandictionary.com/v0/define?term=%s", url.QueryEscape(query)))
		if err != nil {
			return ctx.Reply("[!] Urban Dictionary API offline.")
		}
		defer res.Body.Close()

		var data struct {
			List []struct {
				Word       string `json:"word"`
				Definition string `json:"definition"`
				Example    string `json:"example"`
				Permalink  string `json:"permalink"`
				ThumbsUp   int    `json:"thumbs_up"`
				ThumbsDown int    `json:"thumbs_down"`
			} `json:"list"`
		}

		if err := json.NewDecoder(res.Body).Decode(&data); err != nil || len(data.List) == 0 {
			return ctx.Reply(fmt.Sprintf("[!] No results for `%s`.", query))
		}

		item := data.List[0]
		def := item.Definition
		def = strings.ReplaceAll(def, "[", "")
		def = strings.ReplaceAll(def, "]", "")
		if len(def) > 1000 {
			def = def[:997] + "..."
		}

		ex := item.Example
		ex = strings.ReplaceAll(ex, "[", "")
		ex = strings.ReplaceAll(ex, "]", "")
		if len(ex) > 500 {
			ex = ex[:497] + "..."
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       fmt.Sprintf("Urban: %s", item.Word),
			Description: fmt.Sprintf("%s\n\n*Example:*\n%s", def, ex),
		})
		emb.URL = item.Permalink
		emb.Fields = []*discordgo.MessageEmbedField{
			{
				Name:   "Rating",
				Value:  fmt.Sprintf("👍 %d | 👎 %d", item.ThumbsUp, item.ThumbsDown),
				Inline: true,
			},
		}
		return ctx.Respond(emb)
	},
}
