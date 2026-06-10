package fun

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("randomip", []manager.HelpPage{
		{
			Command:     "Random IP Generator",
			Syntax:      ".randomip",
			Description: "Generates a random valid IPv4 address.",
		},
	})
	manager.RegisterHelp("duckduckgo", []manager.HelpPage{
		{
			Command:     "DuckDuckGo Search",
			Syntax:      ".duckduckgo <query>",
			Description: "Search DuckDuckGo with instant results.",
		},
	})
	manager.RegisterHelp("ocr", []manager.HelpPage{
		{
			Command:     "Optical Character Recognition",
			Syntax:      ".ocr [reply to image message]",
			Description: "Extracts text from an image attachment.",
		},
	})
	manager.RegisterHelp("ocrtr", []manager.HelpPage{
		{
			Command:     "OCR & Translate",
			Syntax:      ".ocrtr <target_lang> [reply to image message]",
			Description: "Extracts text from an image and translates it.",
		},
	})
	manager.RegisterHelp("palette", []manager.HelpPage{
		{
			Command:     "Color Palette Extractor",
			Syntax:      ".palette [reply to image message]",
			Description: "Extracts the dominant colors from an image.",
		},
	})
	manager.RegisterHelp("steal", []manager.HelpPage{
		{
			Command:     "Steal Emoji",
			Syntax:      ".steal <emoji> [name]",
			Description: "Adds a custom emoji from another server to this server.",
		},
	})
	manager.RegisterHelp("weather", []manager.HelpPage{
		{
			Command:     "Weather Search",
			Syntax:      ".weather <location>",
			Description: "Get weather information for any location.",
		},
	})
}

var RandomIP = &manager.Command{
	Trigger:     "randomip",
	Aliases:     []string{"rip", "genip"},
	Name:        "randomip",
	Description: "Generates a random IPv4 address",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		ip := fmt.Sprintf("%d.%d.%d.%d", rand.Intn(223)+1, rand.Intn(256), rand.Intn(256), rand.Intn(256))
		return ctx.Reply(fmt.Sprintf("[+] Generated IP: `%s`", ip))
	},
}

var DuckDuckGo = &manager.Command{
	Trigger:     "duckduckgo",
	Aliases:     []string{"ddg"},
	Name:        "duckduckgo",
	Description: "Search DuckDuckGo with instant results",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("duckduckgo")
		}
		query := strings.Join(ctx.Args, " ")
		apiURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1", url.QueryEscape(query))
		res, err := http.Get(apiURL)
		if err != nil {
			return ctx.Reply("[!] DDG service offline.")
		}
		defer res.Body.Close()

		var data struct {
			AbstractText string `json:"AbstractText"`
			AbstractURL  string `json:"AbstractURL"`
			Heading      string `json:"Heading"`
		}

		_ = json.NewDecoder(res.Body).Decode(&data)

		if data.AbstractText == "" {
			return ctx.Reply(fmt.Sprintf("[*] No instant answer found. Try direct search: https://duckduckgo.com/?q=%s", url.QueryEscape(query)))
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       data.Heading,
			Description: data.AbstractText,
		})
		emb.URL = data.AbstractURL
		return ctx.Respond(emb)
	},
}

func getImgURL(ctx *manager.CommandContext) string {
	if ctx.Message == nil {
		return ""
	}
	if len(ctx.Message.Attachments) > 0 {
		return ctx.Message.Attachments[0].URL
	}
	if ctx.Message.ReferencedMessage != nil && len(ctx.Message.ReferencedMessage.Attachments) > 0 {
		return ctx.Message.ReferencedMessage.Attachments[0].URL
	}
	return ""
}

func doOCR(imgURL string) (string, error) {
	apiURL := fmt.Sprintf("https://api.ocr.space/parse/imageurl?apikey=helloworld&url=%s", url.QueryEscape(imgURL))
	res, err := http.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var data struct {
		ParsedResults []struct {
			ParsedText string `json:"ParsedText"`
		} `json:"ParsedResults"`
		ErrorMessage []string `json:"ErrorMessage"`
	}

	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", err
	}

	if len(data.ParsedResults) > 0 {
		return data.ParsedResults[0].ParsedText, nil
	}

	if len(data.ErrorMessage) > 0 {
		return "", fmt.Errorf("%s", data.ErrorMessage[0])
	}

	return "", fmt.Errorf("OCR parse failed")
}

var OCR = &manager.Command{
	Trigger:     "ocr",
	Name:        "ocr",
	Description: "Extracts text from an image attachment",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		imgURL := getImgURL(ctx)
		if imgURL == "" {
			return ctx.Reply("[!] Please attach or reply to a message containing an image.")
		}

		_ = ctx.Reply("[*] Parsing image, please wait...")
		txt, err := doOCR(imgURL)
		if err != nil || strings.TrimSpace(txt) == "" {
			return ctx.Reply("[!] No readable text found or OCR space limit hit.")
		}

		if len(txt) > 2000 {
			txt = txt[:1997] + "..."
		}
		return ctx.Reply(fmt.Sprintf("```\n%s\n```", txt))
	},
}

var OCRTR = &manager.Command{
	Trigger:     "ocrtr",
	Name:        "ocrtr",
	Description: "Extracts text from an image and translates it",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("ocrtr")
		}
		lang := ctx.Args[0]
		imgURL := getImgURL(ctx)
		if imgURL == "" {
			return ctx.Reply("[!] Please attach or reply to a message containing an image.")
		}

		_ = ctx.Reply("[*] Parsing and translating image...")
		txt, err := doOCR(imgURL)
		if err != nil || strings.TrimSpace(txt) == "" {
			return ctx.Reply("[!] No readable text found in the image.")
		}

		apiURL := fmt.Sprintf("https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=%s&dt=t&q=%s", lang, url.QueryEscape(txt))
		res, err := http.Get(apiURL)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] OCR succeeded, but translation failed.\nText:\n```\n%s\n```", txt))
		}
		defer res.Body.Close()

		var data []interface{}
		_ = json.NewDecoder(res.Body).Decode(&data)

		outer, ok := data[0].([]interface{})
		if !ok || len(outer) == 0 {
			return ctx.Reply(fmt.Sprintf("[!] OCR text:\n```\n%s\n```", txt))
		}

		var parts []string
		for _, item := range outer {
			inner, ok := item.([]interface{})
			if ok && len(inner) > 0 {
				if str, ok := inner[0].(string); ok {
					parts = append(parts, str)
				}
			}
		}

		translated := strings.Join(parts, "")
		if len(translated) > 1900 {
			translated = translated[:1897] + "..."
		}
		return ctx.Reply(fmt.Sprintf("**OCR Translation (%s):**\n```\n%s\n```", lang, translated))
	},
}

var Palette = &manager.Command{
	Trigger:     "palette",
	Aliases:     []string{"colors"},
	Name:        "palette",
	Description: "Extract dominant color palette from image",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		imgURL := getImgURL(ctx)
		if imgURL == "" {
			return ctx.Reply("[!] Please attach or reply to a message containing an image.")
		}

		resp, err := http.Get(imgURL)
		if err != nil {
			return ctx.Reply("[!] Failed to download image.")
		}
		defer resp.Body.Close()

		img, _, err := image.Decode(resp.Body)
		if err != nil {
			return ctx.Reply("[!] Invalid image format. Must be PNG or JPEG.")
		}

		bounds := img.Bounds()
		counts := make(map[string]int)

		for i := 0; i < 1000; i++ {
			x := rand.Intn(bounds.Max.X-bounds.Min.X) + bounds.Min.X
			y := rand.Intn(bounds.Max.Y-bounds.Min.Y) + bounds.Min.Y
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			rVal := r >> 8
			gVal := g >> 8
			bVal := b >> 8

			rRounded := (rVal / 32) * 32
			gRounded := (gVal / 32) * 32
			bRounded := (bVal / 32) * 32

			hex := fmt.Sprintf("#%02x%02x%02x", rRounded, gRounded, bRounded)
			counts[hex]++
		}

		type colorEntry struct {
			hex   string
			count int
		}
		var sorted []colorEntry
		for h, c := range counts {
			sorted = append(sorted, colorEntry{hex: h, count: c})
		}

		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i].count < sorted[j].count {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}

		limit := 5
		if len(sorted) < limit {
			limit = len(sorted)
		}

		var hexes []string
		for i := 0; i < limit; i++ {
			hexes = append(hexes, sorted[i].hex)
		}

		colorBlocks := ""
		for _, h := range hexes {
			colorBlocks += fmt.Sprintf("`%s` \n", h)
		}

		chartCfg := fmt.Sprintf(`{
			type: 'bar',
			data: {
				labels: %s,
				datasets: [{
					data: [1, 1, 1, 1, 1],
					backgroundColor: %s
				}]
			},
			options: {
				legend: { display: false },
				scales: {
					yAxes: [{ display: false }],
					xAxes: [{ ticks: { fontColor: '#fff', fontSize: 16 } }]
				}
			}
		}`, func() string {
			b, _ := json.Marshal(hexes)
			return string(b)
		}(), func() string {
			b, _ := json.Marshal(hexes)
			return string(b)
		}())

		quickChartURL := fmt.Sprintf("https://quickchart.io/chart?width=500&height=100&c=%s", url.QueryEscape(chartCfg))
		imgResp, err := http.Get(quickChartURL)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[+] Dominant Colors:\n%s", colorBlocks))
		}
		defer imgResp.Body.Close()

		chartData, _ := io.ReadAll(imgResp.Body)
		_, err = ctx.Session.ChannelMessageSendComplex(ctx.ChanID(), &discordgo.MessageSend{
			Content: fmt.Sprintf("[+] Dominant Colors:\n%s", colorBlocks),
			Files: []*discordgo.File{
				{
					Name:        "palette.png",
					ContentType: "image/png",
					Reader:      bytes.NewReader(chartData),
				},
			},
		})
		return err
	},
}

var rxEmoji = regexp.MustCompile(`<a?:([a-zA-Z0-9_]+):(\d+)>`)

var Steal = &manager.Command{
	Trigger:     "steal",
	Aliases:     []string{"enlarge", "jumbo"},
	Name:        "steal",
	Description: "Steals emoji or sticker from a message",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
		if err != nil || (p&discordgo.PermissionManageGuildExpressions) == 0 {
			return ctx.Reply("[!] You need Manage Emojis/Stickers permission to steal emojis.")
		}

		targetText := ""
		if len(ctx.Args) > 0 {
			targetText = ctx.Args[0]
		} else if ctx.Message != nil && ctx.Message.ReferencedMessage != nil {
			targetText = ctx.Message.ReferencedMessage.Content
		}

		if targetText == "" {
			return ctx.SendHelp("steal")
		}

		match := rxEmoji.FindStringSubmatch(targetText)
		if len(match) < 3 {
			return ctx.Reply("[!] Could not parse custom emoji from query or referenced message.")
		}

		eName := match[1]
		eID := match[2]
		isAnimated := strings.Contains(match[0], "<a:")

		ext := "png"
		if isAnimated {
			ext = "gif"
		}

		emojiURL := fmt.Sprintf("https://cdn.discordapp.com/emojis/%s.%s", eID, ext)
		resp, err := http.Get(emojiURL)
		if err != nil {
			return ctx.Reply("[!] Failed to fetch emoji source image.")
		}
		defer resp.Body.Close()

		imgBytes, err := io.ReadAll(resp.Body)
		if err != nil || len(imgBytes) == 0 {
			return ctx.Reply("[!] Failed to process emoji image.")
		}

		contentType := "image/png"
		if isAnimated {
			contentType = "image/gif"
		}

		b64 := "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(imgBytes)
		newEmoji, err := ctx.Session.GuildEmojiCreate(ctx.GuildID(), &discordgo.EmojiParams{
			Name:  eName,
			Image: b64,
		})
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to add emoji to server: %v", err))
		}

		return ctx.Reply(fmt.Sprintf("[+] Successfully stole and added emoji %s as `%s`!", newEmoji.MessageFormat(), newEmoji.Name))
	},
}

var Weather = &manager.Command{
	Trigger:     "weather",
	Name:        "weather",
	Description: "Provides detailed weather information for any location",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("weather")
		}
		loc := strings.Join(ctx.Args, " ")
		apiURL := fmt.Sprintf("https://wttr.in/%s?format=j1", url.QueryEscape(loc))
		res, err := http.Get(apiURL)
		if err != nil {
			return ctx.Reply("[!] Weather API offline.")
		}
		defer res.Body.Close()

		var data struct {
			Current []struct {
				TempC     string `json:"temp_C"`
				TempF     string `json:"temp_F"`
				Humidity  string `json:"humidity"`
				WindSpeed string `json:"windspeedKmph"`
				Desc      []struct {
					Value string `json:"value"`
				} `json:"weatherDesc"`
			} `json:"current_condition"`
			Area []struct {
				Name []struct {
					Value string `json:"value"`
				} `json:"areaName"`
				Region []struct {
					Value string `json:"value"`
				} `json:"region"`
				Country []struct {
					Value string `json:"value"`
				} `json:"country"`
			} `json:"nearest_area"`
		}

		if err := json.NewDecoder(res.Body).Decode(&data); err != nil || len(data.Current) == 0 {
			return ctx.Reply(fmt.Sprintf("[!] Weather info for `%s` not found.", loc))
		}

		cur := data.Current[0]
		desc := "Unknown"
		if len(cur.Desc) > 0 {
			desc = cur.Desc[0].Value
		}

		place := loc
		if len(data.Area) > 0 {
			a := data.Area[0]
			var parts []string
			if len(a.Name) > 0 && a.Name[0].Value != "" {
				parts = append(parts, a.Name[0].Value)
			}
			if len(a.Region) > 0 && a.Region[0].Value != "" {
				parts = append(parts, a.Region[0].Value)
			}
			if len(a.Country) > 0 && a.Country[0].Value != "" {
				parts = append(parts, a.Country[0].Value)
			}
			if len(parts) > 0 {
				place = strings.Join(parts, ", ")
			}
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       fmt.Sprintf("Weather in %s", place),
			Description: fmt.Sprintf("**Condition:** %s\n**Temperature:** %s°C / %s°F\n**Humidity:** %s%%\n**Wind Speed:** %s km/h", desc, cur.TempC, cur.TempF, cur.Humidity, cur.WindSpeed),
		})
		return ctx.Respond(emb)
	},
}

