package moderation

import (
	"skyvern/internal/manager"

	"github.com/bwmarrin/discordgo"
)

func checkPerm(ctx *manager.CommandContext, perm int64) bool {
	uid := ctx.AuthorID()
	if uid == "" {
		return false
	}
	if isOwnerOrBypassed(ctx) {
		return true
	}
	p, err := ctx.Session.UserChannelPermissions(uid, "")
	if err != nil {
		return false
	}
	if (p & discordgo.PermissionAdministrator) != 0 {
		return true
	}
	return (p & perm) == perm
}

func isOwner(ctx *manager.CommandContext) bool {
	uid := ctx.AuthorID()
	gid := ctx.GuildID()
	g, err := ctx.Session.State.Guild(gid)
	if err != nil {
		g, err = ctx.Session.Guild(gid)
	}
	return err == nil && g.OwnerID == uid
}

func isOwnerOrBypassed(ctx *manager.CommandContext) bool {
	if isOwner(ctx) {
		return true
	}
	return ctx.DB.HasBypass(ctx.GuildID(), ctx.AuthorID())
}

func checkHierarchy(ctx *manager.CommandContext, tid string) bool {
	gid := ctx.GuildID()
	mid := ctx.AuthorID()
	bid := ctx.Session.State.User.ID

	g, err := ctx.Session.State.Guild(gid)
	if err == nil && g.OwnerID == mid {
		return true
	}

	roles, err := ctx.Session.GuildRoles(gid)
	if err != nil {
		return false
	}
	posMap := make(map[string]int)
	for _, r := range roles {
		posMap[r.ID] = r.Position
	}

	maxRole := func(uid string) int {
		mem, err := ctx.Session.State.Member(gid, uid)
		if err != nil {
			mem, err = ctx.Session.GuildMember(gid, uid)
			if err != nil {
				return -1
			}
		}
		max := -1
		for _, r := range mem.Roles {
			if pos, ok := posMap[r]; ok && pos > max {
				max = pos
			}
		}
		return max
	}

	mMax := maxRole(mid)
	tMax := maxRole(tid)
	bMax := maxRole(bid)

	return mMax > tMax && bMax > tMax
}
