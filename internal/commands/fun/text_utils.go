package fun

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("ascii", []manager.HelpPage{
		{
			Command:     "ASCII Art",
			Syntax:      ".ascii <text>",
			Description: "Converts text into ASCII block art.",
		},
	})
	manager.RegisterHelp("owoify", []manager.HelpPage{
		{
			Command:     "OwO-ify",
			Syntax:      ".owoify <text>",
			Description: "Turns normal text into cute OwO-speak.",
		},
	})
	manager.RegisterHelp("piglatin", []manager.HelpPage{
		{
			Command:     "Pig Latin",
			Syntax:      ".piglatin <text>",
			Description: "Converts text to Pig Latin.",
		},
	})
	manager.RegisterHelp("translate", []manager.HelpPage{
		{
			Command:     "Translate Text",
			Syntax:      ".translate <target_lang> <text>",
			Description: "Translates text using Google Translate.",
		},
	})
	manager.RegisterHelp("tts", []manager.HelpPage{
		{
			Command:     "Text-to-Speech",
			Syntax:      ".tts <text>",
			Description: "Generates a TTS audio file and sends it.",
		},
	})
	manager.RegisterHelp("qr", []manager.HelpPage{
		{
			Command:     "QR Code Generator",
			Syntax:      ".qr <text or url>",
			Description: "Generates a QR code image for any text.",
		},
	})
	manager.RegisterHelp("shorten", []manager.HelpPage{
		{
			Command:     "URL Shortener",
			Syntax:      ".shorten <url>",
			Description: "Shortens a URL using is.gd.",
		},
	})
}

var ASCII = &manager.Command{
	Trigger:     "ascii",
	Name:        "ascii",
	Description: "Converts text to ASCII block art",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("ascii")
		}
		query := strings.Join(ctx.Args, " ")
		res, err := http.Get(fmt.Sprintf("https://asciified.thelicato.io/api/v2/ascii?text=%s", url.QueryEscape(query)))
		if err != nil {
			return ctx.Reply("[!] ASCII API offline.")
		}
		defer res.Body.Close()

		art, _ := io.ReadAll(res.Body)
		if len(art) == 0 {
			return ctx.Reply("[!] Conversion failed.")
		}
		return ctx.Reply(fmt.Sprintf("```\n%s\n```", string(art)))
	},
}

var Owoify = &manager.Command{
	Trigger:     "owoify",
	Name:        "owoify",
	Description: "Turns text into owo speak",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("owoify")
		}
		text := strings.Join(ctx.Args, " ")
		text = strings.ReplaceAll(text, "r", "w")
		text = strings.ReplaceAll(text, "l", "w")
		text = strings.ReplaceAll(text, "R", "W")
		text = strings.ReplaceAll(text, "L", "W")
		text = strings.ReplaceAll(text, "ove", "uv")
		text = text + " o3o"
		return ctx.Reply(text)
	},
}

var Piglatin = &manager.Command{
	Trigger:     "piglatin",
	Name:        "piglatin",
	Description: "Converts text to Pig Latin",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		text := ""
		if len(ctx.Args) > 0 {
			text = strings.Join(ctx.Args, " ")
		} else if ctx.Message != nil && ctx.Message.ReferencedMessage != nil {
			text = ctx.Message.ReferencedMessage.Content
		}

		if text == "" {
			return ctx.SendHelp("piglatin")
		}

		words := strings.Fields(text)
		var out []string
		for _, w := range words {
			if len(w) == 0 {
				continue
			}
			lower := strings.ToLower(w)
			vowels := "aeiou"
			if strings.ContainsRune(vowels, rune(lower[0])) {
				out = append(out, w+"way")
			} else {
				out = append(out, w[1:]+string(w[0])+"ay")
			}
		}
		return ctx.Reply(strings.Join(out, " "))
	},
}

var Translate = &manager.Command{
	Trigger:     "translate",
	Aliases:     []string{"tr"},
	Name:        "translate",
	Description: "Translate text to any language",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) < 2 {
			return ctx.SendHelp("translate")
		}
		lang := ctx.Args[0]
		query := strings.Join(ctx.Args[1:], " ")

		apiURL := fmt.Sprintf("https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=%s&dt=t&q=%s", lang, url.QueryEscape(query))
		res, err := http.Get(apiURL)
		if err != nil {
			return ctx.Reply("[!] Translation service offline.")
		}
		defer res.Body.Close()

		var data []interface{}
		if err := json.NewDecoder(res.Body).Decode(&data); err != nil || len(data) == 0 {
			return ctx.Reply("[!] Translation failed.")
		}

		outer, ok := data[0].([]interface{})
		if !ok || len(outer) == 0 {
			return ctx.Reply("[!] Translation parsing error.")
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

		return ctx.Reply(fmt.Sprintf("[*] **Translation (%s):** %s", lang, strings.Join(parts, "")))
	},
}

var TTS = &manager.Command{
	Trigger:     "tts",
	Name:        "tts",
	Description: "Generates TTS mp3 file",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("tts")
		}
		query := strings.Join(ctx.Args, " ")
		apiURL := fmt.Sprintf("https://translate.google.com/translate_tts?ie=UTF-8&tl=en&client=tw-ob&q=%s", url.QueryEscape(query))
		res, err := http.Get(apiURL)
		if err != nil {
			return ctx.Reply("[!] TTS service offline.")
		}
		defer res.Body.Close()

		data, err := io.ReadAll(res.Body)
		if err != nil || len(data) == 0 {
			return ctx.Reply("[!] Failed to generate TTS audio.")
		}

		_, err = ctx.Session.ChannelMessageSendComplex(ctx.ChanID(), &discordgo.MessageSend{
			Files: []*discordgo.File{
				{
					Name:        "tts.mp3",
					ContentType: "audio/mpeg",
					Reader:      bytes.NewReader(data),
				},
			},
		})
		return err
	},
}

var QR = &manager.Command{
	Trigger:     "qr",
	Name:        "qr",
	Description: "Generates a QR code",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("qr")
		}
		query := strings.Join(ctx.Args, " ")
		apiURL := fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=250x250&data=%s", url.QueryEscape(query))
		res, err := http.Get(apiURL)
		if err != nil {
			return ctx.Reply("[!] QR service offline.")
		}
		defer res.Body.Close()

		data, err := io.ReadAll(res.Body)
		if err != nil || len(data) == 0 {
			return ctx.Reply("[!] Failed to generate QR code.")
		}

		_, err = ctx.Session.ChannelMessageSendComplex(ctx.ChanID(), &discordgo.MessageSend{
			Files: []*discordgo.File{
				{
					Name:        "qr.png",
					ContentType: "image/png",
					Reader:      bytes.NewReader(data),
				},
			},
		})
		return err
	},
}

var Shorten = &manager.Command{
	Trigger:     "shorten",
	Name:        "shorten",
	Description: "Shortens a URL using is.gd",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("shorten")
		}
		target := ctx.Args[0]
		res, err := http.Get(fmt.Sprintf("https://is.gd/create.php?format=simple&url=%s", url.QueryEscape(target)))
		if err != nil {
			return ctx.Reply("[!] URL shortener offline.")
		}
		defer res.Body.Close()

		shortURL, _ := io.ReadAll(res.Body)
		ret := string(shortURL)
		if strings.HasPrefix(ret, "Error:") {
			return ctx.Reply(fmt.Sprintf("[!] Shortening failed: %s", ret))
		}
		return ctx.Reply(fmt.Sprintf("[+] Short URL: %s", ret))
	},
}
