package utility

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"skyvern/internal/manager"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("ticker", []manager.HelpPage{
		{
			Command:     "Ticker Chart",
			Syntax:      ".ticker <symbol>",
			Description: "View a professional 24h price chart for a cryptocurrency.",
		},
	})
}

var Ticker = &manager.Command{
	Trigger:     "ticker",
	Aliases:     []string{"chart", "graph"},
	Name:        "ticker",
	Description: "View a 24h price chart for a cryptocurrency",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("ticker")
		}

		symbol := strings.ToUpper(ctx.Args[0])
		if !strings.HasSuffix(symbol, "USDT") {
			symbol = symbol + "USDT"
		}

		_ = ctx.Reply(fmt.Sprintf("[*] Fetching 24h market chart for %s...", symbol))

		apiURL := fmt.Sprintf("https://api.binance.com/api/v3/klines?symbol=%s&interval=1h&limit=24", symbol)
		resp, err := http.Get(apiURL)
		if err != nil {
			return ctx.Reply("[!] Failed to connect to market data provider.")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return ctx.Reply(fmt.Sprintf("[!] Symbol `%s` is not supported or market data is unavailable.", ctx.Args[0]))
		}

		var klines [][]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&klines); err != nil || len(klines) == 0 {
			return ctx.Reply("[!] Failed to parse market data.")
		}

		var prices []float64
		var labels []string

		for i, k := range klines {
			if len(k) < 5 {
				continue
			}
			closePriceStr, ok := k[4].(string)
			if !ok {
				continue
			}
			price, err := strconv.ParseFloat(closePriceStr, 64)
			if err == nil {
				prices = append(prices, price)
				if i%4 == 0 {
					labels = append(labels, fmt.Sprintf("-%dh", 24-i))
				} else {
					labels = append(labels, "")
				}
			}
		}

		if len(prices) == 0 {
			return ctx.Reply("[!] No valid price data returned.")
		}

		labelsJson, _ := json.Marshal(labels)
		pricesJson, _ := json.Marshal(prices)

		chartCfg := fmt.Sprintf(`{
			type: 'line',
			data: {
				labels: %s,
				datasets: [{
					label: '%s Price (USDT)',
					data: %s,
					fill: true,
					backgroundColor: 'rgba(114, 137, 218, 0.1)',
					borderColor: 'rgba(114, 137, 218, 1)',
					borderWidth: 3,
					pointRadius: 0
				}]
			},
			options: {
				title: {
					display: true,
					text: '24-Hour Price History'
				},
				scales: {
					yAxes: [{
						ticks: {
							fontColor: '#888'
						}
					}],
					xAxes: [{
						gridLines: {
							display: false
						},
						ticks: {
							fontColor: '#888'
						}
					}]
				}
			}
		}`, labelsJson, symbol, pricesJson)

		quickChartURL := fmt.Sprintf("https://quickchart.io/chart?width=800&height=400&c=%s", url.QueryEscape(chartCfg))
		imgResp, err := http.Get(quickChartURL)
		if err != nil {
			return ctx.Reply("[!] Failed to render chart image.")
		}
		defer imgResp.Body.Close()

		imgData, err := io.ReadAll(imgResp.Body)
		if err != nil || len(imgData) == 0 {
			return ctx.Reply("[!] Failed to read chart image data.")
		}

		fileReader := bytes.NewReader(imgData)
		currentPrice := prices[len(prices)-1]
		priceChange := ((currentPrice - prices[0]) / prices[0]) * 100

		changeSign := ""
		if priceChange > 0 {
			changeSign = "+"
		}

		_, err = ctx.Session.ChannelMessageSendComplex(ctx.ChanID(), &discordgo.MessageSend{
			Content: fmt.Sprintf("[*] **Current Price:** $%.4f | **24h Change:** %s%.2f%%", currentPrice, changeSign, priceChange),
			Files: []*discordgo.File{
				{
					Name:        "chart.png",
					ContentType: "image/png",
					Reader:      fileReader,
				},
			},
		})
		return err
	},
}
