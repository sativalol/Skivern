package general

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"skyvern/internal/manager"
	"strings"
	"time"
)

var Timezone = &manager.Command{
	Trigger:     "timezone",
	Aliases:     []string{"tz"},
	Name:        "timezone",
	Description: "View or set your timezone",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		uid := ctx.AuthorID()

		if len(ctx.Args) == 0 {
			tz, err := ctx.DB.GetTimezone(uid)
			if err != nil || tz == "" {
				return ctx.Reply("[*] You haven't set a timezone yet. Use `.timezone <city/country/timezone>` to set one.")
			}
			return ctx.Reply(fmt.Sprintf("[*] Your current timezone is `%s`.", tz))
		}

		query := strings.Join(ctx.Args, " ")
		if strings.ToLower(ctx.Args[0]) == "set" && len(ctx.Args) > 1 {
			query = strings.Join(ctx.Args[1:], " ")
		}

		loc, err := time.LoadLocation(query)
		if err == nil && loc != nil {
			err = ctx.DB.SaveTimezone(uid, query)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to save timezone: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Set your timezone to `%s`.", query))
		}

		apiURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1", url.QueryEscape(query))
		resp, err := http.Get(apiURL)
		if err != nil {
			return ctx.Reply("[!] Failed to reach timezone resolution service. Please try again or specify a direct timezone name (e.g., UTC, America/New_York).")
		}
		defer resp.Body.Close()

		var apiRes struct {
			Results []struct {
				Timezone string `json:"timezone"`
				Name     string `json:"name"`
				Country  string `json:"country"`
			} `json:"results"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&apiRes); err != nil || len(apiRes.Results) == 0 {
			return ctx.Reply(fmt.Sprintf("[!] Could not resolve timezone for `%s`. Try a larger nearby city.", query))
		}

		res := apiRes.Results[0]
		resolvedTZ := res.Timezone
		if resolvedTZ == "" {
			return ctx.Reply(fmt.Sprintf("[!] Resolved location `%s` but it does not have an associated timezone.", res.Name))
		}

		loc, err = time.LoadLocation(resolvedTZ)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Resolved timezone `%s` is invalid/unsupported by system.", resolvedTZ))
		}

		err = ctx.DB.SaveTimezone(uid, resolvedTZ)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to save timezone: %v", err))
		}

		return ctx.Reply(fmt.Sprintf("[+] Resolved `%s` (%s) -> Set your timezone to `%s`.", res.Name, res.Country, resolvedTZ))
	},
}
