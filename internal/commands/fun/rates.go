package fun

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("rate", []manager.HelpPage{
		{
			Command:     "Rate User",
			Syntax:      ".rate <type> [@user]",
			Description: "Rate someone (e.g. gayrate, coolrate, beautyrate, hornyrate, sexyrate).",
		},
	})
	manager.RegisterHelp("ship", []manager.HelpPage{
		{
			Command:     "Ship Users",
			Syntax:      ".ship <user1> [user2]",
			Description: "Ship two users to see their compatibility.",
		},
	})
}

var Rate = &manager.Command{
	Trigger:     "rate",
	Aliases: []string{
		"gayrate", "lesbianrate", "coolrate", "smartness", "beautyrate",
		"awkwardrate", "funnyrate", "sexyrate", "hornyrate", "simprate",
		"epicrate", "savage", "chillrate", "kindrate", "crazyrate",
		"hotrate", "shyrate", "nerdrate", "swagrate", "thugrate",
		"cuterate", "beastmode", "tiredrate", "romanticrate", "danceability",
		"hackerlevel", "gamerate", "bosslevel", "clumsyrate", "spicyrate",
		"rizzarate", "dramarate", "luckrate", "basedrate", "procrastinationrate",
		"villainrate", "angelrate",
	},
	Name:        "rate",
	Description: "Rate a user with a gauge chart",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		rateType := ctx.Message.Content
		if strings.HasPrefix(rateType, ".") {
			rateType = strings.TrimPrefix(rateType, ".")
		}
		parts := strings.Fields(rateType)
		typeStr := "cool"
		if len(parts) > 0 {
			typeStr = strings.ToLower(parts[0])
		}

		targetName := ctx.AuthorTag()
		targetID := ctx.AuthorID()
		gid := ctx.GuildID()

		if len(ctx.Args) > 0 {
			t := resolveVouchUser(ctx.Session, gid, ctx.Args[0])
			if t != "" {
				targetID = t
				mem, err := ctx.Session.State.Member(gid, targetID)
				if err == nil && mem.User != nil {
					targetName = mem.User.Username
				}
			} else {
				targetName = strings.Join(ctx.Args, " ")
			}
		}

		h := fnv.New32a()
		_, _ = h.Write([]byte(targetID + typeStr))
		rateVal := int(h.Sum32()%101)

		chartCfg := fmt.Sprintf(`{
			type: 'radialGauge',
			data: {
				datasets: [{
					data: [%d],
					backgroundColor: 'rgba(114, 137, 218, 1)'
				}]
			},
			options: {
				title: {
					display: true,
					text: '%s rating for %s',
					fontSize: 20
				}
			}
		}`, rateVal, strings.Title(typeStr), targetName)

		apiURL := fmt.Sprintf("https://quickchart.io/chart?width=300&height=300&c=%s", url.QueryEscape(chartCfg))
		res, err := http.Get(apiURL)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[*] **Rating:** %s is %d%% %s!", targetName, rateVal, typeStr))
		}
		defer res.Body.Close()

		imgBytes, _ := io.ReadAll(res.Body)
		_, err = ctx.Session.ChannelMessageSendComplex(ctx.ChanID(), &discordgo.MessageSend{
			Content: fmt.Sprintf("[*] **Rating:** %s is %d%% %s!", targetName, rateVal, typeStr),
			Files: []*discordgo.File{
				{
					Name:        "rate.png",
					ContentType: "image/png",
					Reader:      bytes.NewReader(imgBytes),
				},
			},
		})
		return err
	},
}

var Ship = &manager.Command{
	Trigger:     "ship",
	Name:        "ship",
	Description: "Ship compatibility calculator",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("ship")
		}

		name1 := ctx.AuthorTag()
		id1 := ctx.AuthorID()
		name2 := ""
		id2 := ""
		gid := ctx.GuildID()

		if len(ctx.Args) == 1 {
			t := resolveVouchUser(ctx.Session, gid, ctx.Args[0])
			if t != "" {
				id2 = t
				mem, err := ctx.Session.State.Member(gid, id2)
				if err == nil && mem.User != nil {
					name2 = mem.User.Username
				}
			} else {
				name2 = ctx.Args[0]
			}
		} else {
			t1 := resolveVouchUser(ctx.Session, gid, ctx.Args[0])
			if t1 != "" {
				id1 = t1
				mem, err := ctx.Session.State.Member(gid, id1)
				if err == nil && mem.User != nil {
					name1 = mem.User.Username
				}
			} else {
				name1 = ctx.Args[0]
			}

			t2 := resolveVouchUser(ctx.Session, gid, ctx.Args[1])
			if t2 != "" {
				id2 = t2
				mem, err := ctx.Session.State.Member(gid, id2)
				if err == nil && mem.User != nil {
					name2 = mem.User.Username
				}
			} else {
				name2 = ctx.Args[1]
			}
		}

		h := fnv.New32a()
		comb := id1 + id2
		if id2 < id1 {
			comb = id2 + id1
		}
		_, _ = h.Write([]byte(comb))
		shipVal := int(h.Sum32()%101)

		comment := "A match made in heaven! 💖"
		if shipVal < 20 {
			comment = "Not a chance. 😭"
		} else if shipVal < 50 {
			comment = "Maybe just friends. 🤝"
		} else if shipVal < 80 {
			comment = "Cute match! 💞"
		}

		chartCfg := fmt.Sprintf(`{
			type: 'radialGauge',
			data: {
				datasets: [{
					data: [%d],
					backgroundColor: 'rgba(255, 99, 132, 1)'
				}]
			},
			options: {
				title: {
					display: true,
					text: '%s x %s compatibility',
					fontSize: 18
				}
			}
		}`, shipVal, name1, name2)

		apiURL := fmt.Sprintf("https://quickchart.io/chart?width=300&height=300&c=%s", url.QueryEscape(chartCfg))
		res, err := http.Get(apiURL)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("💞 **Ship:** %s x %s = %d%% compatibility!\n*%s*", name1, name2, shipVal, comment))
		}
		defer res.Body.Close()

		imgBytes, _ := io.ReadAll(res.Body)
		_, err = ctx.Session.ChannelMessageSendComplex(ctx.ChanID(), &discordgo.MessageSend{
			Content: fmt.Sprintf("💞 **Ship:** %s x %s = %d%% compatibility!\n*%s*", name1, name2, shipVal, comment),
			Files: []*discordgo.File{
				{
					Name:        "ship.png",
					ContentType: "image/png",
					Reader:      bytes.NewReader(imgBytes),
				},
			},
		})
		return err
	},
}
