package fun

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type rpConfig struct {
	Trigger     string
	NekosCat    string
	WaifuCat    string
	GiphySearch string
	SelfTexts   []string
	TargetTexts []string
}

func fetchGiphy(query string) string {
	u := fmt.Sprintf("https://api.giphy.com/v1/gifs/search?api_key=dc6zaTOxFJmzC&q=%s&limit=20", url.QueryEscape(query))
	resp, err := http.Get(u)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var res struct {
		Data []struct {
			Images struct {
				Original struct {
					URL string `json:"url"`
				} `json:"original"`
			} `json:"images"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || len(res.Data) == 0 {
		return ""
	}
	return res.Data[rand.Intn(len(res.Data))].Images.Original.URL
}

func fetchNekosBest(cat string) string {
	if cat == "" {
		return ""
	}
	resp, err := http.Get("https://nekos.best/api/v2/" + cat)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var res struct {
		Results []struct {
			URL string `json:"url"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || len(res.Results) == 0 {
		return ""
	}
	return res.Results[0].URL
}

func fetchWaifuPics(cat string) string {
	if cat == "" {
		return ""
	}
	resp, err := http.Get("https://api.waifu.pics/sfw/" + cat)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var res struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return ""
	}
	return res.URL
}

func fetchRPImage(catNekos, catWaifu, giphyQuery string) string {
	if catNekos != "" {
		if u := fetchNekosBest(catNekos); u != "" {
			return u
		}
	}
	if catWaifu != "" {
		if u := fetchWaifuPics(catWaifu); u != "" {
			return u
		}
	}
	return fetchGiphy(giphyQuery)
}

func resolveRPTarget(s *discordgo.Session, gid, query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return ""
	}
	if m, err := moderation.ResolveMember(s, gid, q); err == nil && m != nil {
		return "<@" + m.User.ID + ">"
	}
	return q
}

func makeRPCommand(cfg rpConfig) *manager.Command {
	return &manager.Command{
		Trigger:     cfg.Trigger,
		Name:        cfg.Trigger,
		Description: fmt.Sprintf("Perform a %s action on a user.", cfg.Trigger),
		Category:    "Roleplay",
		Execute: func(ctx *manager.CommandContext) error {
			sender := "<@" + ctx.Message.Author.ID + ">"
			target := ""
			if len(ctx.Args) > 0 {
				target = resolveRPTarget(ctx.Session, ctx.Message.GuildID, strings.Join(ctx.Args, " "))
			}

			var txt string
			if target == "" || target == sender {
				txt = fmt.Sprintf(cfg.SelfTexts[rand.Intn(len(cfg.SelfTexts))], sender)
			} else {
				txt = fmt.Sprintf(cfg.TargetTexts[rand.Intn(len(cfg.TargetTexts))], sender, target)
			}

			gif := fetchRPImage(cfg.NekosCat, cfg.WaifuCat, cfg.GiphySearch)
			emb := config.Build(ctx.Cfg, config.EmbedOpt{
				Description: txt,
			})
			if gif != "" {
				emb.Image = &discordgo.MessageEmbedImage{URL: gif}
			}
			return ctx.Respond(emb)
		},
	}
}

// Custom specialized roleplay commands
var Maclookup = &manager.Command{
	Trigger:     "maclookup",
	Aliases:     []string{"mac"},
	Name:        "maclookup",
	Description: "Look up information about a MAC address.",
	Category:    "Roleplay",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: `.maclookup <mac_address>`")
		}
		mac := ctx.Args[0]
		resp, err := http.Get("https://api.macvendors.com/" + url.QueryEscape(mac))
		if err != nil {
			return ctx.Reply("[!] MAC address lookup failed.")
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != 200 {
			return ctx.Reply(fmt.Sprintf("[!] Vendor not found for MAC: %s", mac))
		}
		
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, resp.Body)
		vendor := strings.TrimSpace(buf.String())
		
		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title: "MAC Address Lookup",
			Description: fmt.Sprintf("Results for **%s**", mac),
		})
		emb.Fields = []*discordgo.MessageEmbedField{
			{Name: "MAC Address", Value: mac, Inline: true},
			{Name: "Vendor/Company", Value: vendor, Inline: true},
		}
		return ctx.Respond(emb)
	},
}

var Touch = &manager.Command{
	Trigger:     "touch",
	Name:        "touch",
	Description: "Touch a user and track it on the leaderboard.",
	Category:    "Roleplay",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.Message.GuildID
		sender := ctx.Message.Author.ID
		
		if len(ctx.Args) == 0 {
			list, err := ctx.DB.ListTouches(gid)
			if err != nil || len(list) == 0 {
				return ctx.Reply("[*] No touches recorded yet. Be the first to `.touch` someone!")
			}
			
			sort.Slice(list, func(i, j int) bool {
				return (list[i].Sent + list[i].Recv) > (list[j].Sent + list[j].Recv)
			})
			
			var sb strings.Builder
			limit := len(list)
			if limit > 10 {
				limit = 10
			}
			for i := 0; i < limit; i++ {
				tr := list[i]
				sb.WriteString(fmt.Sprintf("%d. <@%s> — **%d** Touches Given, **%d** Touches Received\n", i+1, tr.UserID, tr.Sent, tr.Recv))
			}
			
			emb := config.Build(ctx.Cfg, config.EmbedOpt{
				Title: "Touch Leaderboard",
				Description: sb.String(),
			})
			return ctx.Respond(emb)
		}
		
		targetMem, err := moderation.ResolveMember(ctx.Session, gid, strings.Join(ctx.Args, " "))
		if err != nil || targetMem == nil {
			return ctx.Reply("[!] Could not resolve target user.")
		}
		target := targetMem.User.ID
		
		sRec, rRec, err := ctx.DB.RecordTouch(gid, sender, target)
		if err != nil {
			return ctx.Reply("[!] Failed to record touch.")
		}
		
		var txt string
		if sender == target {
			txt = fmt.Sprintf("<@%s> touched themselves! (Total Touches: %d)", sender, sRec.Sent)
		} else {
			txt = fmt.Sprintf("<@%s> touched <@%s>! (Given: %d, Received: %d)", sender, target, sRec.Sent, rRec.Recv)
		}
		
		gif := fetchRPImage("", "poke", "anime poke")
		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Description: txt,
		})
		if gif != "" {
			emb.Image = &discordgo.MessageEmbedImage{URL: gif}
		}
		return ctx.Respond(emb)
	},
}

// Generate the standard roleplay commands from templates
var standardRPConfigs = []rpConfig{
	{
		Trigger: "airkiss", GiphySearch: "anime airkiss",
		SelfTexts: []string{"%s blew a kiss into the air!"},
		TargetTexts: []string{"%s blew a sweet airkiss to %s! 💋"},
	},
	{
		Trigger: "angrystare", GiphySearch: "anime angry stare",
		SelfTexts: []string{"%s is staring angrily into the void."},
		TargetTexts: []string{"%s is staring angrily at %s! 😡"},
	},
	{
		Trigger: "bark", GiphySearch: "anime bark",
		SelfTexts: []string{"%s barks at the walls! Woof!"},
		TargetTexts: []string{"%s barks at %s! Woof woof!"},
	},
	{
		Trigger: "bite", NekosCat: "bite", WaifuCat: "bite", GiphySearch: "anime bite",
		SelfTexts: []string{"%s is biting their own lip."},
		TargetTexts: []string{"%s bites %s! Ouch!"},
	},
	{
		Trigger: "bleh", GiphySearch: "anime bleh",
		SelfTexts: []string{"%s goes: Bleh! 😜"},
		TargetTexts: []string{"%s sticks their tongue out at %s! Bleh!"},
	},
	{
		Trigger: "brofist", GiphySearch: "anime brofist",
		SelfTexts: []string{"%s raises a fist for a brofist."},
		TargetTexts: []string{"%s gives a solid brofist to %s! 👊"},
	},
	{
		Trigger: "celebrate", GiphySearch: "anime celebrate",
		SelfTexts: []string{"%s is celebrating!"},
		TargetTexts: []string{"%s celebrates with %s! 🎉"},
	},
	{
		Trigger: "cheers", GiphySearch: "anime cheers",
		SelfTexts: []string{"%s raises their glass!"},
		TargetTexts: []string{"%s cheers with %s! 🍻"},
	},
	{
		Trigger: "clap", GiphySearch: "anime clap",
		SelfTexts: []string{"%s claps their hands!"},
		TargetTexts: []string{"%s applauds %s! 👏"},
	},
	{
		Trigger: "confused", GiphySearch: "anime confused",
		SelfTexts: []string{"%s looks completely confused."},
		TargetTexts: []string{"%s looks confusedly at %s."},
	},
	{
		Trigger: "cool", GiphySearch: "anime cool",
		SelfTexts: []string{"%s puts on sunglasses."},
		TargetTexts: []string{"%s acts cool around %s. 😎"},
	},
	{
		Trigger: "cry", NekosCat: "cry", WaifuCat: "cry", GiphySearch: "anime cry",
		SelfTexts: []string{"%s is crying... 😭"},
		TargetTexts: []string{"%s cries on %s's shoulder."},
	},
	{
		Trigger: "cuddle", NekosCat: "cuddle", WaifuCat: "cuddle", GiphySearch: "anime cuddle",
		SelfTexts: []string{"%s wants cuddles."},
		TargetTexts: []string{"%s cuddles %s warmly!"},
	},
	{
		Trigger: "dance", NekosCat: "dance", WaifuCat: "dance", GiphySearch: "anime dance",
		SelfTexts: []string{"%s starts dancing!"},
		TargetTexts: []string{"%s dances with %s! 💃"},
	},
	{
		Trigger: "drool", GiphySearch: "anime drool",
		SelfTexts: []string{"%s is drooling..."},
		TargetTexts: []string{"%s drools over %s."},
	},
	{
		Trigger: "evillaugh", GiphySearch: "anime evil laugh",
		SelfTexts: []string{"%s laughs evilly! Muahaha!"},
		TargetTexts: []string{"%s laughs evilly at %s!"},
	},
	{
		Trigger: "facepalm", GiphySearch: "anime facepalm",
		SelfTexts: []string{"%s facepalms."},
		TargetTexts: []string{"%s facepalms at %s."},
	},
	{
		Trigger: "fuck", GiphySearch: "anime intimate",
		SelfTexts: []string{"%s wants to get intimate."},
		TargetTexts: []string{"%s gets intimate with %s."},
	},
	{
		Trigger: "grabbreast", GiphySearch: "anime chest grab",
		SelfTexts: []string{"%s is feeling playful."},
		TargetTexts: []string{"%s grabs %s by the breast! (playful)"},
	},
	{
		Trigger: "grabwaist", GiphySearch: "anime grab waist",
		SelfTexts: []string{"%s grabs their own waist."},
		TargetTexts: []string{"%s grabs %s by the waist! (playful)"},
	},
	{
		Trigger: "handhold", WaifuCat: "handhold", GiphySearch: "anime handhold",
		SelfTexts: []string{"%s wants to hold hands."},
		TargetTexts: []string{"%s holds hands with %s! ❤️"},
	},
	{
		Trigger: "happy", NekosCat: "happy", WaifuCat: "happy", GiphySearch: "anime happy",
		SelfTexts: []string{"%s is super happy! 😄"},
		TargetTexts: []string{"%s is happy for %s!"},
	},
	{
		Trigger: "headbang", GiphySearch: "anime headbang",
		SelfTexts: []string{"%s is headbanging to the music!"},
		TargetTexts: []string{"%s headbangs with %s!"},
	},
	{
		Trigger: "laugh", NekosCat: "laugh", GiphySearch: "anime laugh",
		SelfTexts: []string{"%s bursts into laughter! 😂"},
		TargetTexts: []string{"%s laughs at %s!"},
	},
	{
		Trigger: "lick", WaifuCat: "lick", GiphySearch: "anime lick",
		SelfTexts: []string{"%s licks their own lips."},
		TargetTexts: []string{"%s licks %s!"},
	},
	{
		Trigger: "love", GiphySearch: "anime love",
		SelfTexts: []string{"%s sends love!"},
		TargetTexts: []string{"%s showers %s with love! 💕"},
	},
	{
		Trigger: "mad", GiphySearch: "anime mad",
		SelfTexts: []string{"%s is mad!"},
		TargetTexts: []string{"%s is mad at %s! 😠"},
	},
	{
		Trigger: "meow", GiphySearch: "anime meow",
		SelfTexts: []string{"%s goes: Meow! 🐱"},
		TargetTexts: []string{"%s meows at %s!"},
	},
	{
		Trigger: "nervous", GiphySearch: "anime nervous",
		SelfTexts: []string{"%s is sweating nervously."},
		TargetTexts: []string{"%s is nervous around %s."},
	},
	{
		Trigger: "nom", WaifuCat: "nom", GiphySearch: "anime nom",
		SelfTexts: []string{"%s is nomming on some food."},
		TargetTexts: []string{"%s noms on %s!"},
	},
	{
		Trigger: "nuzzle", GiphySearch: "anime nuzzle",
		SelfTexts: []string{"%s wants a nuzzle."},
		TargetTexts: []string{"%s nuzzles %s affectionately!"},
	},
	{
		Trigger: "nyah", GiphySearch: "anime nyah",
		SelfTexts: []string{"%s goes: Nyah! ~"},
		TargetTexts: []string{"%s nyahs at %s!"},
	},
	{
		Trigger: "pat", NekosCat: "pat", WaifuCat: "pat", GiphySearch: "anime pat",
		SelfTexts: []string{"%s pats themselves on the head."},
		TargetTexts: []string{"%s gently pats %s on the head!"},
	},
	{
		Trigger: "peek", GiphySearch: "anime peek",
		SelfTexts: []string{"%s is peeking."},
		TargetTexts: []string{"%s peeks at %s."},
	},
	{
		Trigger: "pinch", GiphySearch: "anime pinch",
		SelfTexts: []string{"%s pinches their own cheek."},
		TargetTexts: []string{"%s pinches %s's cheek!"},
	},
	{
		Trigger: "poke", NekosCat: "poke", WaifuCat: "poke", GiphySearch: "anime poke",
		SelfTexts: []string{"%s is poking the air."},
		TargetTexts: []string{"%s pokes %s!"},
	},
	{
		Trigger: "pout", NekosCat: "pout", WaifuCat: "pout", GiphySearch: "anime pout",
		SelfTexts: []string{"%s pouts."},
		TargetTexts: []string{"%s pouts at %s."},
	},
	{
		Trigger: "punch", GiphySearch: "anime punch",
		SelfTexts: []string{"%s punches the air."},
		TargetTexts: []string{"%s punches %s! 👊"},
	},
	{
		Trigger: "rape", GiphySearch: "anime intimate",
		SelfTexts: []string{"%s is feeling intimate."},
		TargetTexts: []string{"%s gets intimate with %s."},
	},
	{
		Trigger: "sad", GiphySearch: "anime sad",
		SelfTexts: []string{"%s is feeling sad... 😞"},
		TargetTexts: []string{"%s feels sad with %s."},
	},
	{
		Trigger: "scared", GiphySearch: "anime scared",
		SelfTexts: []string{"%s is scared!"},
		TargetTexts: []string{"%s is scared of %s!"},
	},
	{
		Trigger: "shout", GiphySearch: "anime shout",
		SelfTexts: []string{"%s shouts out loud!"},
		TargetTexts: []string{"%s shouts at %s!"},
	},
	{
		Trigger: "shrug", NekosCat: "shrug", GiphySearch: "anime shrug",
		SelfTexts: []string{"%s shrugs."},
		TargetTexts: []string{"%s shrugs at %s."},
	},
	{
		Trigger: "shy", GiphySearch: "anime shy",
		SelfTexts: []string{"%s is feeling shy..."},
		TargetTexts: []string{"%s is acting shy around %s."},
	},
	{
		Trigger: "sigh", GiphySearch: "anime sigh",
		SelfTexts: []string{"%s sighs."},
		TargetTexts: []string{"%s sighs at %s."},
	},
	{
		Trigger: "sip", GiphySearch: "anime sip",
		SelfTexts: []string{"%s takes a sip of their drink."},
		TargetTexts: []string{"%s sips a drink with %s."},
	},
	{
		Trigger: "slap", NekosCat: "slap", WaifuCat: "slap", GiphySearch: "anime slap",
		SelfTexts: []string{"%s slaps themselves (wake up!)."},
		TargetTexts: []string{"%s slaps %s! Ouch!"},
	},
	{
		Trigger: "sleep", NekosCat: "sleep", GiphySearch: "anime sleep",
		SelfTexts: []string{"%s goes to sleep. Zzz..."},
		TargetTexts: []string{"%s sleeps next to %s."},
	},
	{
		Trigger: "slowclap", GiphySearch: "anime slow clap",
		SelfTexts: []string{"%s claps slowly."},
		TargetTexts: []string{"%s slowclaps at %s."},
	},
	{
		Trigger: "smack", GiphySearch: "anime smack",
		SelfTexts: []string{"%s smacks the desk."},
		TargetTexts: []string{"%s smacks %s!"},
	},
	{
		Trigger: "smile", NekosCat: "smile", WaifuCat: "smile", GiphySearch: "anime smile",
		SelfTexts: []string{"%s smiles warmly. 😊"},
		TargetTexts: []string{"%s smiles at %s."},
	},
	{
		Trigger: "smug", NekosCat: "smug", WaifuCat: "smug", GiphySearch: "anime smug",
		SelfTexts: []string{"%s smugs."},
		TargetTexts: []string{"%s smugs at %s."},
	},
	{
		Trigger: "sneeze", GiphySearch: "anime sneeze",
		SelfTexts: []string{"%s sneezes! Achoo!"},
		TargetTexts: []string{"%s sneezes on %s!"},
	},
	{
		Trigger: "sorry", GiphySearch: "anime sorry",
		SelfTexts: []string{"%s says sorry to the void."},
		TargetTexts: []string{"%s apologizes to %s."},
	},
	{
		Trigger: "stare", NekosCat: "stare", GiphySearch: "anime stare",
		SelfTexts: []string{"%s stares."},
		TargetTexts: []string{"%s stares at %s."},
	},
	{
		Trigger: "surprised", GiphySearch: "anime surprised",
		SelfTexts: []string{"%s is surprised!"},
		TargetTexts: []string{"%s is surprised by %s!"},
	},
	{
		Trigger: "sweat", GiphySearch: "anime sweat",
		SelfTexts: []string{"%s is sweating."},
		TargetTexts: []string{"%s sweats around %s."},
	},
	{
		Trigger: "thumbsup", NekosCat: "thumbsup", GiphySearch: "anime thumbs up",
		SelfTexts: []string{"%s gives a thumbs up!"},
		TargetTexts: []string{"%s gives %s a thumbs up! 👍"},
	},
	{
		Trigger: "tickle", NekosCat: "tickle", GiphySearch: "anime tickle",
		SelfTexts: []string{"%s tickles themselves (weird)."},
		TargetTexts: []string{"%s tickles %s! 😄"},
	},
	{
		Trigger: "tired", GiphySearch: "anime tired",
		SelfTexts: []string{"%s is tired..."},
		TargetTexts: []string{"%s is tired of %s."},
	},
	{
		Trigger: "wave", NekosCat: "wave", WaifuCat: "wave", GiphySearch: "anime wave",
		SelfTexts: []string{"%s waves."},
		TargetTexts: []string{"%s waves at %s! 👋"},
	},
	{
		Trigger: "wink", NekosCat: "wink", WaifuCat: "wink", GiphySearch: "anime wink",
		SelfTexts: []string{"%s winks."},
		TargetTexts: []string{"%s winks at %s! 😉"},
	},
	{
		Trigger: "woah", GiphySearch: "anime woah",
		SelfTexts: []string{"%s goes: Woah!"},
		TargetTexts: []string{"%s goes: Woah! at %s."},
	},
	{
		Trigger: "yawn", GiphySearch: "anime yawn",
		SelfTexts: []string{"%s yawns."},
		TargetTexts: []string{"%s yawns with %s."},
	},
	{
		Trigger: "yay", GiphySearch: "anime yay",
		SelfTexts: []string{"%s cheers: Yay!"},
		TargetTexts: []string{"%s cheers: Yay! with %s."},
	},
	{
		Trigger: "yes", GiphySearch: "anime yes",
		SelfTexts: []string{"%s nods: Yes."},
		TargetTexts: []string{"%s nods yes to %s."},
	},
}

// Registry exported commands slice
var RoleplayCommands []*manager.Command

func init() {
	RoleplayCommands = append(RoleplayCommands, Maclookup, Touch)
	for _, rp := range standardRPConfigs {
		RoleplayCommands = append(RoleplayCommands, makeRPCommand(rp))
	}
}
