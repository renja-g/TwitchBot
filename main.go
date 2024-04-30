package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gempir/go-twitch-irc/v4"
)

const (
	username = "XXXXXX"
	token    = "oauth:XXXXXX"
	channel  = "XXXXXX"

	summonerGameName = "leo5000twitch"
	summonerTagLine = "EUW"
	summonerPlatform = "euw1"
	riotApiKey = "RGAPI-XXXXXX"
)
var summonerId string
var err error


type LeagueEntry struct {
	LeagueId     string `json:"leagueId"`
	QueueType    string `json:"queueType"`
	Tier         string `json:"tier"`
	Rank         string `json:"rank"`
	SummonerId   string `json:"summonerId"`
	LeaguePoints int    `json:"leaguePoints"`
	Wins         int    `json:"wins"`
	Losses       int    `json:"losses"`
	Veteran      bool   `json:"veteran"`
	Inactive     bool   `json:"inactive"`
	FreshBlood   bool   `json:"freshBlood"`
	HotStreak    bool   `json:"hotStreak"`
}

type account struct {
	Puuid string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine string `json:"tagLine"`
}

type Summoner struct {
	AccountId string `json:"accountId"`
	ProfileIconId int `json:"profileIconId"`
	RevisionDate int `json:"revisionDate"`
	Id string `json:"id"`
	Puuid string `json:"puuid"`
	SummonerLevel int `json:"summonerLevel"`
}

func main() {
	summonerId, err = getSummonerId(summonerGameName, summonerTagLine)
	if err != nil {
		log.Fatalf("Error getting summonerId: %v", err)
	}

	var wg sync.WaitGroup

	twClient := twitch.NewClient(username, token)

	twClient.OnPrivateMessage(func(message twitch.PrivateMessage) {
		fmt.Println(message.Message)
		if message.Message[0] == '!' {
			handleCommand(message.Message, twClient, channel)
		}
	})

	twClient.Join(channel)

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := twClient.Connect()
		if err != nil {
			log.Fatalf("Error connecting to Twitch: %v", err)
		}
	}()

	fmt.Println("Connected to Twitch")

	wg.Wait()
}

func handleCommand(message string, client *twitch.Client, channel string) {
	switch {
	case strings.HasPrefix(message, "!commands"):
		sendMessage(client, channel, "Commands: !opgg, !rank")
	
	case strings.HasPrefix(message, "!opgg"):
		sendMessage(client, channel, fmt.Sprintf("https://www.op.gg/summoners/%s/%s-%s", summonerPlatform, summonerGameName, summonerTagLine))

	case strings.HasPrefix(message, "!rank"):
		handleRankCommand(client, channel)
	}
}

func sendMessage(client *twitch.Client, channel string, message string) {
	client.Say(channel, message)
	fmt.Println(message)
}

func handleRankCommand(client *twitch.Client, channel string) {
	lolUrl := fmt.Sprintf("https://%s.api.riotgames.com/lol/league/v4/entries/by-summoner/%s?api_key=%s",summonerPlatform, summonerId, riotApiKey)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, lolUrl, nil)
	if err != nil {
		log.Errorf("Error creating request: %v", err)
		return
	}
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorf("Error making request to Riot Games API: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Error reading response body: %v", err)
		return
	}

	var lolEntries []LeagueEntry
	err = json.Unmarshal(body, &lolEntries)
	if err != nil {
		log.Errorf("Error unmarshalling JSON: %v", err)
		return
	}

	tftUrl := fmt.Sprintf("https://%s.api.riotgames.com/tft/league/v1/entries/by-summoner/%s?api_key=%s",summonerPlatform, summonerId, riotApiKey)
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, tftUrl, nil)
	if err != nil {
		log.Errorf("Error creating request: %v", err)
		return
	}

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		log.Errorf("Error making request to Riot Games API: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Error reading response body: %v", err)
		return
	}

	var tftEntries []LeagueEntry
	err = json.Unmarshal(body, &tftEntries)
	if err != nil {
		log.Errorf("Error unmarshalling JSON: %v", err)
		return
	}

	var solo, flex *LeagueEntry
	for _, entry := range lolEntries {
		if entry.QueueType == "RANKED_SOLO_5x5" {
			solo = &entry
		} else if entry.QueueType == "RANKED_FLEX_SR" {
			flex = &entry
		}
	}

	var tftSolo, tftDoubleUp *LeagueEntry
	for _, entry := range tftEntries {
		if entry.QueueType == "RANKED_TFT" {
			tftSolo = &entry
		} else if entry.QueueType == "RANKED_TFT_DOUBLE_UP" {
			tftDoubleUp = &entry
		}
	}

	if solo != nil {
		sendMessage(client, channel, fmt.Sprintf("Solo: %s %s, %d LP, %dW %dL", solo.Tier, solo.Rank, solo.LeaguePoints, solo.Wins, solo.Losses))
	}
	if flex != nil {
		sendMessage(client, channel, fmt.Sprintf("Flex: %s %s, %d LP, %dW %dL", flex.Tier, flex.Rank, flex.LeaguePoints, flex.Wins, flex.Losses))
	}
	if tftSolo != nil {
		sendMessage(client, channel, fmt.Sprintf("TFT Solo: %s %s, %d LP, %dW %dL", tftSolo.Tier, tftSolo.Rank, tftSolo.LeaguePoints, tftSolo.Wins, tftSolo.Losses))
	}
	if tftDoubleUp != nil {
		sendMessage(client, channel, fmt.Sprintf("TFT Double Up: %s %s, %d LP, %dW %dL", tftDoubleUp.Tier, tftDoubleUp.Rank, tftDoubleUp.LeaguePoints, tftDoubleUp.Wins, tftDoubleUp.Losses))
	}
	if solo == nil && flex == nil && tftSolo == nil && tftDoubleUp == nil {
		sendMessage(client, channel, "No ranked data found")
	}
}


func getSummonerId(gameName string, tagLine string) (string, error) {
	// 1. get puuid /riot/account/v1/accounts/by-riot-id/{gameName}/{tagLine}
	// 2. get summonerId /summoner/v4/summoners/by-puuid/{puuid}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	riotUrl := fmt.Sprintf("https://europe.api.riotgames.com/riot/account/v1/accounts/by-riot-id/%s/%s?api_key=%s", gameName, tagLine, riotApiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, riotUrl, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request to Riot Games API: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	var account account
	err = json.Unmarshal(body, &account)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling JSON: %v", err)
	}


	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	riotUrl = fmt.Sprintf("https://%s.api.riotgames.com/lol/summoner/v4/summoners/by-puuid/%s?api_key=%s", summonerPlatform, account.Puuid, riotApiKey)
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, riotUrl, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request to Riot Games API: %v", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	var summoner Summoner
	err = json.Unmarshal(body, &summoner)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling JSON: %v", err)
	}

	fmt.Println(summoner.Id)
	return summoner.Id, nil
}
