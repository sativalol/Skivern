package utility

import (
	"encoding/json"
	"fmt"
	"net/http"
	"skyvern/internal/manager"
)

var IP = &manager.Command{
	Trigger:     "ip",
	Aliases:     []string{"ipcheck", "iplookup"},
	Name:        "ip",
	Description: "Lookup security and location info for an IP address",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: `.ip <ip_address>`")
		}

		ip := ctx.Args[0]
		apiURL := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,country,regionName,city,zip,timezone,isp,as,query", ip)

		resp, err := http.Get(apiURL)
		if err != nil {
			return ctx.Reply("[!] Failed to connect to IP lookup service.")
		}
		defer resp.Body.Close()

		var res struct {
			Status     string `json:"status"`
			Message    string `json:"message"`
			Country    string `json:"country"`
			RegionName string `json:"regionName"`
			City       string `json:"city"`
			Zip        string `json:"zip"`
			Timezone   string `json:"timezone"`
			Isp        string `json:"isp"`
			As         string `json:"as"`
			Query      string `json:"query"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || res.Status != "success" {
			errMsg := "Invalid IP or lookup failed."
			if res.Message != "" {
				errMsg = res.Message
			}
			return ctx.Reply(fmt.Sprintf("[!] Lookup failed: %s", errMsg))
		}

		info := fmt.Sprintf("**IP Information for %s**\n\n"+
			"**Location:** %s, %s, %s (Zip: %s)\n"+
			"**Timezone:** %s\n"+
			"**ISP:** %s\n"+
			"**ASN:** %s",
			res.Query, res.City, res.RegionName, res.Country, res.Zip, res.Timezone, res.Isp, res.As)

		return ctx.Reply(info)
	},
}
