package config

import "github.com/bwmarrin/discordgo"

type EmbedOpt struct {
	Title        string
	Description  string
	Fields       []*discordgo.MessageEmbedField
	ImageURL     string
	ThumbnailURL string
}

func Build(cfg ResCfg, opt EmbedOpt) *discordgo.MessageEmbed {
	var footerIcon string
	if cfg.ShowLogo {
		footerIcon = cfg.FooterIcon
	}

	e := &discordgo.MessageEmbed{
		Color:       cfg.EmbedColor,
		Description: opt.Description,
		Footer: &discordgo.MessageEmbedFooter{
			Text:    cfg.Footer,
			IconURL: footerIcon,
		},
	}
	if opt.Title != "" {
		e.Title = opt.Title
	}
	if len(opt.Fields) > 0 {
		e.Fields = opt.Fields
	}
	if opt.ImageURL != "" {
		e.Image = &discordgo.MessageEmbedImage{URL: opt.ImageURL}
	}

	if cfg.ShowLogo {
		if opt.Title != "" || len(opt.Fields) > 0 || opt.ThumbnailURL != "" {
			thumb := opt.ThumbnailURL
			if thumb == "" {
				thumb = cfg.FooterIcon
			}
			if thumb != "" {
				e.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: thumb}
			}
		}
	}

	return e
}

func Wrap(cfg ResCfg, d string) *discordgo.MessageEmbed {
	return Build(cfg, EmbedOpt{Description: d})
}

func Field(name, v string, inline bool) *discordgo.MessageEmbedField {
	return &discordgo.MessageEmbedField{Name: name, Value: v, Inline: inline}
}
