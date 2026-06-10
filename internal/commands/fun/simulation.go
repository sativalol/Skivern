package fun

import (
	"fmt"
	"math/rand"
	"skyvern/internal/manager"
	"skyvern/internal/storage"
	"strings"
	"sync"
	"time"
)

var (
	bluntRot   = make(map[string][]string)
	bluntIndex = make(map[string]int)
	bluntHits  = make(map[string]int)
	bluntMu    sync.Mutex

	juulCharge = make(map[string]int)
	juulPods   = make(map[string]int)
	juulMu     sync.Mutex

	yartCharge = make(map[string]int)
	yartHits   = make(map[string]int)
	yartMu     sync.Mutex
)

func init() {
	manager.RegisterHelp("blunt", []manager.HelpPage{
		{
			Command:     "Blunt Rotation",
			Syntax:      ".blunt OR .blunt add <user> OR .blunt pass",
			Description: "Smoke the blunt, pass it, or add people to the rotation.",
		},
	})
	manager.RegisterHelp("juul", []manager.HelpPage{
		{
			Command:     "Share Juul",
			Syntax:      ".juul OR .juul charge OR .juul pod",
			Description: "Take a hit of the Juul, charge it, or replace the pod.",
		},
	})
	manager.RegisterHelp("yart", []manager.HelpPage{
		{
			Command:     "Virtual Weed Pen",
			Syntax:      ".yart OR .yart blinker OR .yart charge",
			Description: "Take a puff, hit a 10s blinker, or charge the yart.",
		},
	})
	manager.RegisterHelp("weed", []manager.HelpPage{
		{
			Command:     "Grow Weed Plant",
			Syntax:      ".weed OR .weed water OR .weed fertilize OR .weed harvest",
			Description: "Nurture and grow a server-wide weed plant.",
		},
	})
}

var Blunt = &manager.Command{
	Trigger:     "blunt",
	Aliases:     []string{"smoke", "sesh"},
	Name:        "blunt",
	Description: "Smoke and pass the blunt",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()
		uid := ctx.AuthorID()
		bluntMu.Lock()
		defer bluntMu.Unlock()

		if len(ctx.Args) > 0 {
			sub := strings.ToLower(ctx.Args[0])
			if sub == "add" {
				if len(ctx.Args) < 2 {
					return ctx.Reply("[!] Specify a user to add.")
				}
				target := resolveVouchUser(ctx.Session, gid, ctx.Args[1])
				if target == "" {
					return ctx.Reply("[!] User not found.")
				}
				bluntRot[gid] = append(bluntRot[gid], target)
				return ctx.Reply(fmt.Sprintf("[+] Added <@%s> to the session rotation.", target))
			}
			if sub == "pass" {
				rot := bluntRot[gid]
				if len(rot) <= 1 {
					return ctx.Reply("[!] Not enough people in the rotation. Use `.blunt add <user>`.")
				}
				bluntIndex[gid] = (bluntIndex[gid] + 1) % len(rot)
				next := rot[bluntIndex[gid]]
				return ctx.Reply(fmt.Sprintf("[*] Blunt passed. It is now <@%s>'s turn to hit.", next))
			}
		}

		rot := bluntRot[gid]
		if len(rot) == 0 {
			bluntRot[gid] = []string{uid}
			rot = bluntRot[gid]
			bluntIndex[gid] = 0
		}

		current := rot[bluntIndex[gid]]
		if current != uid {
			return ctx.Reply(fmt.Sprintf("[!] It is not your turn in the rotation. Currently <@%s>'s turn.", current))
		}

		bluntHits[gid]++
		hits := bluntHits[gid]

		if hits >= 5 {
			bluntHits[gid] = 0
			bluntRot[gid] = nil
			bluntIndex[gid] = 0
			return ctx.Reply("💨 *Cough! Cough!* The blunt is finished. Start a new session with `.blunt`.")
		}

		return ctx.Reply(fmt.Sprintf("💨 <@%s> takes a hit of the blunt. (%d/5 hits remaining)", uid, 5-hits))
	},
}

var Juul = &manager.Command{
	Trigger:     "juul",
	Name:        "juul",
	Description: "Share a juul with friends",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()
		uid := ctx.AuthorID()
		juulMu.Lock()
		defer juulMu.Unlock()

		if _, ok := juulCharge[gid]; !ok {
			juulCharge[gid] = 100
			juulPods[gid] = 100
		}

		if len(ctx.Args) > 0 {
			sub := strings.ToLower(ctx.Args[0])
			if sub == "charge" {
				juulCharge[gid] = 100
				return ctx.Reply("[+] Juul charged to 100% 🔋")
			}
			if sub == "pod" {
				juulPods[gid] = 100
				return ctx.Reply("[+] Slapped in a fresh mint pod 🧪")
			}
		}

		if juulCharge[gid] <= 0 {
			return ctx.Reply("[!] The Juul is dead. Charge it with `.juul charge`.")
		}
		if juulPods[gid] <= 0 {
			return ctx.Reply("[!] The pod is empty. Replace it with `.juul pod`.")
		}

		hitCharge := rand.Intn(10) + 5
		hitPod := rand.Intn(8) + 4

		juulCharge[gid] -= hitCharge
		juulPods[gid] -= hitPod

		if juulCharge[gid] < 0 {
			juulCharge[gid] = 0
		}
		if juulPods[gid] < 0 {
			juulPods[gid] = 0
		}

		return ctx.Reply(fmt.Sprintf("💨 <@%s> takes a hit of the Juul.\n🔋 **Battery:** %d%% | 🧪 **Pod:** %d%%", uid, juulCharge[gid], juulPods[gid]))
	},
}

var Yart = &manager.Command{
	Trigger:     "yart",
	Name:        "yart",
	Description: "Virtual weed pen",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()
		uid := ctx.AuthorID()
		yartMu.Lock()
		defer yartMu.Unlock()

		if _, ok := yartCharge[gid]; !ok {
			yartCharge[gid] = 100
			yartHits[gid] = 200
		}

		if len(ctx.Args) > 0 {
			sub := strings.ToLower(ctx.Args[0])
			if sub == "charge" {
				yartCharge[gid] = 100
				return ctx.Reply("[+] Weed pen fully charged. 🔋")
			}
		}

		if yartCharge[gid] <= 0 {
			return ctx.Reply("[!] Weed pen is dead. Charge it with `.yart charge`.")
		}
		if yartHits[gid] <= 0 {
			return ctx.Reply("[!] Cartridge is completely empty.")
		}

		if len(ctx.Args) > 0 && strings.ToLower(ctx.Args[0]) == "blinker" {
			yartCharge[gid] -= 25
			yartHits[gid] -= 20
			if yartCharge[gid] < 0 {
				yartCharge[gid] = 0
			}
			if yartHits[gid] < 0 {
				yartHits[gid] = 0
			}
			return ctx.Reply(fmt.Sprintf("💨 🔟💨 <@%s> hits a 10s blinker!\n🔋 **Battery:** %d%% | 🍯 **Oil:** %d%%", uid, yartCharge[gid], (yartHits[gid]*100)/200))
		}

		yartCharge[gid] -= 5
		yartHits[gid] -= 3
		if yartCharge[gid] < 0 {
			yartCharge[gid] = 0
		}
		if yartHits[gid] < 0 {
			yartHits[gid] = 0
		}

		return ctx.Reply(fmt.Sprintf("💨 <@%s> takes a pull from the weed pen.\n🔋 **Battery:** %d%% | 🍯 **Oil:** %d%%", uid, yartCharge[gid], (yartHits[gid]*100)/200))
	},
}

var Weed = &manager.Command{
	Trigger:     "weed",
	Name:        "weed",
	Description: "Nurture and grow a server weed plant",
	Category:    "fun",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()
		uid := ctx.AuthorID()

		wp, err := ctx.DB.GetWeedPlant(gid)
		if err != nil {
			wp = storage.WeedPlant{
				Growth:     0,
				Water:      50,
				Fertilizer: 50,
				LastAction: time.Now(),
			}
		}

		elapsed := time.Since(wp.LastAction).Hours()
		if elapsed > 0.05 {
			growthFactor := elapsed * 5.0
			if wp.Water > 0 && wp.Fertilizer > 0 {
				wp.Growth += growthFactor
				wp.Water -= elapsed * 8.0
				wp.Fertilizer -= elapsed * 6.0
			} else {
				wp.Growth -= elapsed * 2.0
			}

			if wp.Growth > 100 {
				wp.Growth = 100
			}
			if wp.Growth < 0 {
				wp.Growth = 0
			}
			if wp.Water < 0 {
				wp.Water = 0
			}
			if wp.Fertilizer < 0 {
				wp.Fertilizer = 0
			}
			wp.LastAction = time.Now()
			_ = ctx.DB.SaveWeedPlant(gid, wp)
		}

		if len(ctx.Args) > 0 {
			sub := strings.ToLower(ctx.Args[0])
			if sub == "water" {
				wp.Water = 100
				wp.LastAction = time.Now()
				_ = ctx.DB.SaveWeedPlant(gid, wp)
				return ctx.Reply("[+] Watered the plant! 💧")
			}
			if sub == "fertilize" {
				wp.Fertilizer = 100
				wp.LastAction = time.Now()
				_ = ctx.DB.SaveWeedPlant(gid, wp)
				return ctx.Reply("[+] Fertilized the plant! 🧪")
			}
			if sub == "harvest" {
				if wp.Growth < 100 {
					return ctx.Reply(fmt.Sprintf("[!] Plant is not ready for harvest yet (Current growth: %.1f%%).", wp.Growth))
				}
				yield := rand.Intn(15) + 10
				wp.Growth = 0
				wp.Water = 30
				wp.Fertilizer = 30
				wp.LastAction = time.Now()
				_ = ctx.DB.SaveWeedPlant(gid, wp)
				return ctx.Reply(fmt.Sprintf("🌿 <@%s> harvested the plant and yielded **%d grams** of top-shelf bud!", uid, yield))
			}
		}

		status := "Seed"
		if wp.Growth > 20 {
			status = "Sprout"
		}
		if wp.Growth > 50 {
			status = "Vegetative"
		}
		if wp.Growth > 80 {
			status = "Flowering"
		}
		if wp.Growth >= 100 {
			status = "Ready to Harvest!"
		}

		return ctx.Reply(fmt.Sprintf("🌿 **Weed Plant Status:**\n"+
			"**Growth:** %.1f%% (%s)\n"+
			"💧 **Water:** %.1f%%\n"+
			"🧪 **Fertilizer:** %.1f%%\n\n"+
			"Use `.weed water`, `.weed fertilize`, or `.weed harvest`.",
			wp.Growth, status, wp.Water, wp.Fertilizer))
	},
}
