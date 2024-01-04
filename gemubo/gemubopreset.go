package gemubo

import (
	"errors"
	"fmt"
	"gemubobot/lib"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Preset struct {
	Name     string
	Template *Template
	Params   map[string]string
}

type GemuboMessage struct {
	Content   string
	StartTime *time.Time
	GuildId   string
	ChannelId string
	GemuboId  string
	MessgeId  string
	Author    *discordgo.User
	ImageURL  string
	Title     string
}

func NewPreset(name string, template *Template, params map[string]string) *Preset {
	return &Preset{
		Name:     name,
		Template: template,
		Params:   params,
	}
}

func parseTime(str string) (*time.Time, error) {
	tokens := strings.Split(str, ":")
	hour, err := strconv.Atoi(tokens[0])
	if err != nil {
		return nil, err
	}

	minu, err := strconv.Atoi(tokens[1])
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	nowJapan := now.Add(time.Hour * 9)

	targetTime := time.Date(nowJapan.Year(), nowJapan.Month(), nowJapan.Day(), hour, minu, 0, 0, time.UTC)
	if targetTime.Before(nowJapan) {
		targetTime = targetTime.Add(time.Hour * 24)
	}
	targetTime = targetTime.Add(time.Hour * -9)
	return &targetTime, nil
}

func (p *Preset) MakeMessage(additonalParam map[string]string, channelId string, guildID string, author *discordgo.User) (*GemuboMessage, error) {
	msg := p.Template.Content

	params := make(map[string]string)
	for pname, value := range p.Params {
		params[pname] = value
	}
	if additonalParam != nil {
		for pname, value := range additonalParam {
			params[pname] = value
		}
	}

	gmsg := &GemuboMessage{
		Content:   "",
		StartTime: nil,
		ChannelId: channelId,
		MessgeId:  "",
		GemuboId:  lib.GeneRandomID(),
		GuildId:   guildID,
		Author:    author,
		ImageURL:  "",
		Title:     "",
	}

	START_TIME := "$START_TIME"
	TITLE := "$TITLE"
	IMAGE_URL := "$IMAGE_URL"

	for pname, value := range params {
		switch pname {
		case START_TIME:
			if value != "NOW" {
				t, err := parseTime(value)
				if err != nil {
					return nil, errors.New("Error: 時間は\"hh:mm\"で指定してください")
				}
				gmsg.StartTime = t
			}
		case TITLE:
			gmsg.Title = value
		case IMAGE_URL:
			gmsg.ImageURL = value
			gmsg.ImageURL = strings.TrimLeft(gmsg.ImageURL, "<")
			gmsg.ImageURL = strings.TrimRight(gmsg.ImageURL, ">")
		}

		pstr := pname
		msg = strings.ReplaceAll(msg, pstr, value)
	}

	gmsg.Content = msg
	fmt.Println("gmsg GuilID:", gmsg.GuildId)
	return gmsg, nil
}

func MakeEmbedBosyuMessage(gmsg *GemuboMessage) *discordgo.MessageEmbed {

	msg := ""
	msg += fmt.Sprintf("ID:%s\n", gmsg.GemuboId)
	msg += "─────────────────────────────\n"

	texts := strings.Split(gmsg.Content, "\n")
	for _, text := range texts {
		if text == "" {
			continue
		}
		msg += "### " + text + "\n"
	}

	embed := &discordgo.MessageEmbed{
		Title:       gmsg.Title,
		Description: msg,
		Color:       0x00F1AA,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    gmsg.Author.Username,
			IconURL: gmsg.Author.AvatarURL("128"),
		},
		Image: &discordgo.MessageEmbedImage{
			URL: gmsg.ImageURL,
		},
	}

	if embed.Title == "" {
		embed.Title = fmt.Sprintf("%sがゲムボ！", gmsg.Author.Username)
	}

	return embed
}
