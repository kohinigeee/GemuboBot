package botmanager

import (
	"fmt"
	"gemubobot/gemubo"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	globalManager *BotManager = nil
)

type CommandArg struct {
	s           *discordgo.Session
	m           *discordgo.MessageCreate
	token       []string
	originalMsg string
	commandName string
}

func NewCommandArg(s *discordgo.Session, m *discordgo.MessageCreate, token []string, originalMsg, commandName string) *CommandArg {
	return &CommandArg{
		s:           s,
		m:           m,
		token:       token,
		originalMsg: originalMsg,
		commandName: commandName,
	}
}

type Command struct {
	Name    string
	handler func(arg *CommandArg, manager *BotManager)
	summary string
	detail  string
}

type BotManager struct {
	discordSession    *discordgo.Session
	BotUserInfo       *discordgo.User
	presets           map[string]*gemubo.Preset
	templates         map[string]*gemubo.Template
	bosyuMsgs         map[string]*gemubo.GemuboMessage
	commands          map[string]*Command
	batchDurationMinu int
	lastBatchDate     time.Time
	nextBatchDate     time.Time
	OkReaction        string
	NoReaction        string
}

func NewBotManager(discordSession *discordgo.Session) *BotManager {
	manager := &BotManager{
		discordSession:    discordSession,
		BotUserInfo:       nil,
		presets:           make(map[string]*gemubo.Preset),
		templates:         make(map[string]*gemubo.Template),
		bosyuMsgs:         make(map[string]*gemubo.GemuboMessage),
		batchDurationMinu: 3,
		lastBatchDate:     time.Now().UTC(),
		OkReaction:        "👍",
		NoReaction:        "🙏",
	}
	manager.setCommands()
	manager.discordSession.AddHandler(onDiscordMessageCreate)
	return manager
}

func SetGlobalManager(manager *BotManager) {
	globalManager = manager
}

func GetGlobalManager() *BotManager {
	return globalManager
}

func (manager *BotManager) Start() {
	manager.BotUserInfo = manager.discordSession.State.User
	go manager.batchLoop()
}

func (manager *BotManager) addGemuboMessage(msg *gemubo.GemuboMessage) {
	if msg.StartTime != nil {
		manager.bosyuMsgs[msg.GemuboId] = msg
	}
}

func (manager *BotManager) BosyuNotion(gemuboId string) {
	gmsg, exist := manager.bosyuMsgs[gemuboId]
	if !exist {
		return
	}

	okUsers, err := manager.discordSession.MessageReactions(gmsg.ChannelId, gmsg.MessgeId, manager.OkReaction, 10, "", "")
	if err != nil {
		log.Println("Error getting reaction users")
		return
	}
	fmt.Printf("Notioned Messge: %+v\n", gmsg)

	msgContent := ""
	msgContent += gmsg.Author.Mention() + " "
	for _, user := range okUsers {
		if user.ID == manager.BotUserInfo.ID {
			continue
		}
		msgContent += user.Mention() + " "
	}
	msgContent += "\n"
	msgTitle := "全員しゅうごう～!"

	embed := &discordgo.MessageEmbed{
		Title:       msgTitle,
		Description: "",
		Color:       0x00F1AA,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: manager.BotUserInfo.AvatarURL("20"),
		},
	}
	embes := make([]*discordgo.MessageEmbed, 0)
	embes = append(embes, embed)

	options := &discordgo.MessageSend{
		Content: msgContent,
		Reference: &discordgo.MessageReference{
			MessageID: gmsg.MessgeId,
		},
		Embeds: embes,
	}

	_, err = manager.discordSession.ChannelMessageSendComplex(gmsg.ChannelId, options)
	if err != nil {
		log.Println("Error sending notion message")
		errmsg := fmt.Sprintf("開始通知の送信に失敗しました\n(ID:%s)", gmsg.GemuboId)
		manager.SendErrorMessage(gmsg.ChannelId, "", errmsg, nil)
		return
	}
}

func (manager *BotManager) batchLoop() {
	for {
		dulation := time.Duration(manager.batchDurationMinu) * time.Minute
		time.Sleep(dulation)
		now := time.Now().UTC()
		log.Println("Batch Executed : ", now.Add(9*time.Hour).Format("2006-01-02 15:04:05"))

		//最終探索時間の更新
		manager.lastBatchDate = time.Now().UTC()
		manager.nextBatchDate = manager.lastBatchDate.Add(dulation)

		//招集

		for _, msg := range manager.bosyuMsgs {
			if msg.StartTime.Before(manager.lastBatchDate) {
				manager.BosyuNotion(msg.GemuboId)
				delete(manager.bosyuMsgs, msg.GemuboId)
			}
		}
	}
}

func (manager *BotManager) setCommands() {
	manager.commands = make(map[string]*Command)
	commands := make([]*Command, 0)
	commands = append(commands, &Command{
		Name:    "help",
		handler: onHelpCommand,
		summary: "コマンド一覧や詳細を表示します",
		detail:  "【コマンド】 " + "\n\t\t**help\t(コマンド名)**\n" + "【機能】\n" + "\t・コマンドの一覧を表示します\n" + "\t・コマンド名を指定すると詳細を表示します\n",
	})
	commands = append(commands, &Command{
		Name:    "settempl",
		handler: onSetTemplateCommand,
		summary: "テンプレートを登録します",
		detail:  "【コマンド】 " + "\n\t\t**settempl\tname=<テンプレート名> <テンプレート内容>**\n" + "【機能】\n" + "\t・募集メッセージのテンプレートを登録します\n" + "\t・テンプレート内容は複数行に渡って指定できます\n" + "\t・$で変数を設定できます\n" + "【テンプレート例】\n" + "\tゲーム: $GAMES\n" + "\t人数: $NUM\n" + "\t開始: $START_TIME\n",
	})
	commands = append(commands, &Command{
		Name:    "templs",
		handler: onTemplatesCommand,
		summary: "テンプレート一覧や詳細を表示します",
		detail:  "【コマンド】 " + "\n\t\t**templs\t(テンプレート名)**\n" + "【機能】\n" + "\t・テンプレートの一覧を表示します\n" + "\t・テンプレート名を指定すると詳細を表示します\n",
	})
	commands = append(commands, &Command{
		Name:    "setpreset",
		handler: onSetPresetCommand,
		summary: "プリセットを登録します",
		detail:  "【コマンド】 " + "\n\t\t**setpreset\ttemplname=<テンプレート名>\tpresetname=<プリセット名>\t(<変数名>=<値>)...**\n" + "【機能】\n" + "\t・募集メッセージのプリセット(テンプレートと変数の値のセット)を登録します\n" + "\t・テンプレート名は「!gemubo templs」で確認できます\n" + "\t・変数名はテンプレートの変数名と同じものを指定してください\n" + "\t・変数名は複数指定できます(全ての変数を指定する必要はありません)\n" + "\t・変数の代入値には半角スペースは使えません(全角スペースを使用してください)\n" + "【コマンド例】\n" + "\tsetpreset" + "\ttemplname=templ1" + "\tpresetname=pre1\n" + "\t$GAMES=valo　OW\n" + "\t$NUM=5\n" + "\t$START_TIME=20:00\n",
	})
	commands = append(commands, &Command{
		Name:    "presets",
		handler: onPresetsCommand,
		summary: "プリセット一覧や詳細を表示します",
		detail:  "【コマンド】 " + "\n\t\t**presets\t(プリセット名)**\n" + "【機能】\n" + "\t・プリセットの一覧を表示します\n" + "\t・プリセット名を指定すると詳細を表示します\n",
	})
	commands = append(commands, &Command{
		Name:    "bosyu",
		handler: onBosyuCommand,
		summary: "募集を行います",
		detail:  "【コマンド】 " + "\n\t\t**bosyu\t<template=<テンプレート名>\t|\tpreset=<プリセット名>>\t(<変数名>=<値>)...**\n" + "【機能】\n" + "\t・テンプレートの変数を代入して募集メッセージを送信します\n" + "\t・テンプレート名かプリセット名はどちらかを必ず指定してください\n" + "\t・変数名はテンプレートの変数名と同じものを指定してください\n" + "\t・変数の代入値には半角スペースは使えません(全角スペースを使用してください)\n" + "\t・$START_TIME変数は特殊であり、時間をhh:mm形式で指定することで開始時刻を設定できます\n" + "\t・$START_TIME変数を指定しないまたは`NOW`を代入することで即時開始となります\n" + "\t・開始時刻時にOKのリアクションを押している人に対して通知を行います\n" + "\t・$IMAGE_URL変数は特殊であり, URLを指定することで任意の画像を添付できます\n" + "\t・$TITLE変数は特殊であり、任意の文字列を募集メッセージのタイトルに設定できます(指定なしの場合はデフォルトのタイトルが使用されます)\n" + "【コマンド例】\n" + "\tbosyu" + "\tpreset=pre1\n" + "\t$NUM=3\n" + "\t$START_TIME=20:30\n",
	})
	commands = append(commands, &Command{
		Name:    "notions",
		handler: onNotionsCommand,
		summary: "募集一覧を表示します",
		detail:  "【コマンド】 " + "\n\t\t**notions**\n" + "【機能】\n" + "\t・現在募集中の募集一覧を表示します\n",
	})
	commands = append(commands, &Command{
		Name:    "remove_notion",
		handler: onRemoveNotion,
		summary: "募集を削除します",
		detail:  "【コマンド】 " + "**\n\t\tremove_notion\t<募集ID>\n**" + "【機能】\n" + "\t・募集IDを指定して募集を削除します\n" + "\t・募集IDは「!gemubo notions」で確認できます\n",
	})
	commands = append(commands, &Command{
		Name:    "remove_preset",
		handler: onRemovePreset,
		summary: "プリセットを削除します",
		detail:  "【コマンド】 " + "**\n\t\tremove_preset\t<プリセット名>\n**" + "【機能】\n" + "\t・プリセット名を指定してプリセットを削除します\n" + "\t・プリセット名は「!gemubo presets」で確認できます\n",
	})
	commands = append(commands, &Command{
		Name:    "remove_templ",
		handler: onRemoveTemplate,
		summary: "テンプレートを削除します",
		detail:  "【コマンド】 " + "**\n\t\tremove_templ\t<テンプレート名>\n**" + "【機能】\n" + "\t・テンプレート名を指定してテンプレートを削除します\n" + "\t・テンプレート名は「!gemubo templs」で確認できます\n" + "\t・テンプレートを削除するとそれに紐づくプリセットも削除されます\n",
	})
	commands = append(commands, &Command{
		Name:    "howuse",
		handler: onHowUseCommand,
		summary: "当BOTの使い方を表示します",
	})
	commands = append(commands, &Command{
		Name:    "test",
		handler: onTestCommand,
		summary: "開発用コマンドです",
	})

	for _, command := range commands {
		manager.commands[command.Name] = command
	}
}

func sendMessage(s *discordgo.Session, channelID, msg string) {
	_, err := s.ChannelMessageSend(channelID, msg)
	if err != nil {
		log.Println("Error sending message")
	}
}

func onDiscordMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	commandTriger := "!gemubo"
	msg := m.Content

	tokens1 := strings.Split(msg, "\n")
	tokens := make([]string, 0)

	tokens = append(tokens, strings.Split(tokens1[0], " ")...)
	for _, token := range tokens1[1:] {
		tokens = append(tokens, token)
	}

	if commandTriger != tokens[0] {
		return
	}

	manager := GetGlobalManager()

	if len(tokens) < 2 {
		onInvalidCommand(s, m, manager)
	}

	commandName := tokens[1]

	commandArg := NewCommandArg(s, m, tokens, msg, commandName)

	//コマンドの実行
	if command, ok := manager.commands[commandName]; ok {
		log.Printf("Execute command: %s", commandName)
		command.handler(commandArg, manager)
	} else {
		fmt.Println("Invalid command: ", commandName)
		onInvalidCommand(s, m, manager)
	}
}

func onHelpCommand(arg *CommandArg, manager *BotManager) {
	if len(arg.token) < 3 {
		keys := make([]string, 0, len(manager.commands))
		for key := range manager.commands {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		msg := "コマンド一覧\n"
		for _, name := range keys {
			command := manager.commands[name]
			msg += fmt.Sprintf("**%s**\n\tー\t%s\n", command.Name, command.summary)
		}
		msg += "各コマンドの詳細は「!gemubo help <コマンド名>」で確認できます\n"
		sendMessage(arg.s, arg.m.ChannelID, msg)
		return
	}

	commandName := arg.token[2]
	command, exist := manager.commands[commandName]
	if !exist {
		errmsg := "指定されたコマンドは存在しません"
		title := arg.commandName
		manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
		return
	}

	msg := command.detail
	sendMessage(arg.s, arg.m.ChannelID, msg)
}

func onSetTemplateCommand(arg *CommandArg, manager *BotManager) {
	contentStardIdx := 3
	params := paramParse(arg.token[2:])

	templateName, exist := params["name"]
	if !exist {
		titiel := arg.commandName
		errmsg := "テンプレート名が指定されていません。"
		manager.SendErrorMessage(arg.m.ChannelID, titiel, errmsg, nil)
		return
	}

	content := ""
	for _, token := range arg.token[contentStardIdx:] {
		if token != "" {
			content += token + "\n"
		}
	}

	if content == "" {
		title := arg.commandName
		errmsg := "テンプレート内容が指定されていません。"
		manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
		return
	}

	content = strings.TrimLeft(content, "\n")
	template := gemubo.NewTemplate(templateName, content)
	manager.templates[templateName] = template
	fmt.Println("Set template: ", templateName)
	msg := fmt.Sprintf("テンプレート「%s」を登録しました。", templateName)
	manager.SendNormalMessage(arg.m.ChannelID, "", msg, nil)
}

func onTemplatesCommand(arg *CommandArg, manager *BotManager) {
	if len(arg.token) < 3 {
		content := ""
		for _, template := range manager.templates {
			content += fmt.Sprintf("-\t%s\n", template.Name)
		}
		fields := make([]*discordgo.MessageEmbedField, 0)
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "テンプレート一覧",
			Value:  content,
			Inline: false,
		})
		manager.SendNormalMessage(arg.m.ChannelID, "", "", fields)
		return
	}

	templateName := arg.token[2]
	template, exist := manager.templates[templateName]
	if !exist {
		title := arg.commandName
		errmsg := "テンプレートが存在しません。"
		manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
		return
	}

	fileds := make([]*discordgo.MessageEmbedField, 0)
	fileds = append(fileds, &discordgo.MessageEmbedField{
		Name:   "テンプレート名",
		Value:  template.Name + "\n",
		Inline: true,
	})
	fileds = append(fileds, &discordgo.MessageEmbedField{
		Name:   "テンプレート内容",
		Value:  template.Content + "\n",
		Inline: true,
	})
	manager.SendNormalMessage(arg.m.ChannelID, "", "", fileds)
}

func onSetPresetCommand(arg *CommandArg, manager *BotManager) {
	params := paramParse(arg.token[2:])

	templateName, exist := params["templname"]
	if !exist {
		title := arg.commandName
		errmsg := "テンプレート名が指定されていません。"
		manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
		return
	}

	presetName, exist := params["presetname"]
	if !exist {
		errmsg := "プリセット名が指定されていません。"
		title := arg.commandName
		manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
		return
	}

	msgParams := make(map[string]string)
	for pname, value := range params {
		if isVariable(pname) {
			msgParams[pname] = value
		}
	}

	template, exist := manager.templates[templateName]
	if !exist {
		errmsg := "テンプレートが存在しません。"
		title := arg.commandName
		manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
		return
	}

	preset := gemubo.NewPreset(presetName, template, msgParams)
	manager.presets[presetName] = preset

	msg := fmt.Sprintf("プリセット「%s」を登録しました。", presetName)
	log.Println(msg)
	manager.SendNormalMessage(arg.m.ChannelID, "", msg, nil)

}

func onPresetsCommand(arg *CommandArg, manager *BotManager) {
	if len(arg.token) < 3 {
		content := ""
		for _, preset := range manager.presets {
			content += fmt.Sprintf("-\t%s\n", preset.Name)
		}
		fields := make([]*discordgo.MessageEmbedField, 0)
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "プリセット一覧",
			Value:  content,
			Inline: false,
		})
		manager.SendNormalMessage(arg.m.ChannelID, "", "", fields)
		return
	}

	presetName := arg.token[2]
	preset, exist := manager.presets[presetName]
	if !exist {
		title := arg.commandName
		errmsg := "プリセットが存在しません。"
		manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
		return
	}

	msg := ""
	for pname, value := range preset.Params {
		msg += fmt.Sprintf("-\t%s = \"%s\"\n", pname, value)
	}
	fileds := make([]*discordgo.MessageEmbedField, 0)
	fileds = append(fileds, &discordgo.MessageEmbedField{
		Name:   "プリセット名",
		Value:  preset.Name + "\n",
		Inline: true,
	})
	fileds = append(fileds, &discordgo.MessageEmbedField{
		Name:   "テンプレート名",
		Value:  preset.Template.Name + "\n",
		Inline: true,
	})
	fileds = append(fileds, &discordgo.MessageEmbedField{
		Name:   "テンプレート変数",
		Value:  msg + "\n",
		Inline: true,
	})
	manager.SendNormalMessage(arg.m.ChannelID, "", "", fileds)
}

func onBosyuCommand(arg *CommandArg, manager *BotManager) {
	params := paramParse(arg.token[2:])
	presetName, exist := params["preset"]
	OkReaction := manager.OkReaction
	NoReaction := manager.NoReaction
	author := arg.m.Author

	//プリセットが指定されている場合
	if exist {
		preset, exist := manager.presets[presetName]
		if !exist {
			title := arg.commandName
			errmsg := "プリセットが存在しません。"
			manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
			return
		}

		additonalParam := make(map[string]string)
		for pname, value := range params {
			if isVariable(pname) {
				additonalParam[pname] = value
			}
		}

		gemuboMsg, err := preset.MakeMessage(additonalParam, arg.m.ChannelID, arg.m.GuildID, author)
		if err != nil {
			title := arg.commandName
			errmsg := fmt.Sprintf("不正な値が指定されています(%s)", err.Error())
			manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
			return
		}

		embed := gemubo.MakeEmbedBosyuMessage(gemuboMsg)
		embeds := make([]*discordgo.MessageEmbed, 0)
		embeds = append(embeds, embed)

		content := "@everyone\n"
		msgObj := &discordgo.MessageSend{
			Content: content,
			Embeds:  embeds,
		}

		dmsg, err := arg.s.ChannelMessageSendComplex(arg.m.ChannelID, msgObj)

		if err != nil {
			log.Println("Error sending embed message")
			title := arg.commandName
			errmsg := fmt.Sprintf("メッセージの送信に失敗しました。\n(ID:%s)", gemuboMsg.GemuboId)
			manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
			return
		}

		gemuboMsg.MessgeId = dmsg.ID
		manager.addGemuboMessage(gemuboMsg)

		arg.s.MessageReactionAdd(arg.m.ChannelID, dmsg.ID, OkReaction)
		arg.s.MessageReactionAdd(arg.m.ChannelID, dmsg.ID, NoReaction)

		return
	}

	//プリセットが指定されていない場合
	templateName, exist := params["template"]
	if !exist {
		title := arg.commandName
		errmsg := "テンプレート名またはプリセット名が指定されていません。"
		manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
		return
	}

	template, exist := manager.templates[templateName]
	if !exist {

		errmsg := "テンプレートが存在しません。"
		title := arg.commandName
		manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
		return
	} else {
		msgParams := make(map[string]string)
		for pname, value := range params {
			if isVariable(pname) {
				msgParams[pname] = value
			}
		}

		preset := gemubo.NewPreset(templateName, template, msgParams)

		gemuboMsg, err := preset.MakeMessage(nil, arg.m.ChannelID, arg.m.GuildID, author)
		if err != nil {
			errmsg := fmt.Sprintf("不正な値が指定されています(%s)", err.Error())
			title := arg.commandName
			manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
			return
		}

		embed := gemubo.MakeEmbedBosyuMessage(gemuboMsg)
		dmsg, err := arg.s.ChannelMessageSendEmbed(arg.m.ChannelID, embed)

		if err != nil {
			fmt.Println("Error sending embed message")
			errmsg := fmt.Sprintf("メッセージの送信に失敗しました\n(ID:%s)", gemuboMsg.GemuboId)
			title := arg.commandName
			manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
			return
		}

		gemuboMsg.MessgeId = dmsg.ID
		manager.addGemuboMessage(gemuboMsg)

		arg.s.MessageReactionAdd(arg.m.ChannelID, dmsg.ID, OkReaction)
		arg.s.MessageReactionAdd(arg.m.ChannelID, dmsg.ID, NoReaction)

		return
	}

}

func onNotionsCommand(arg *CommandArg, manager *BotManager) {
	msg := ""
	for _, gmsg := range manager.bosyuMsgs {
		messageLink := ""
		messageLink = fmt.Sprintf("https://discord.com/channels/%s/%s/%s", gmsg.GuildId, gmsg.ChannelId, gmsg.MessgeId)

		msg += fmt.Sprintf("-\tID: %s ([Content](<%s>))\n", gmsg.GemuboId, messageLink)
		startJPTime := gmsg.StartTime.Add(time.Hour * 9)
		msg += fmt.Sprintf("\t\t\t開始時刻:%s\n", startJPTime.Format("2006-01-02 15:04:05"))
	}
	title := "募集一覧"
	manager.SendNormalMessage(arg.m.ChannelID, title, msg, nil)
}

func onRemoveNotion(arg *CommandArg, manager *BotManager) {
	if len(arg.token) < 3 {
		errmsg := "削除する募集のIDが指定されていません"
		title := arg.commandName
		manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
		return
	}

	gemuboId := arg.token[2]
	_, exist := manager.bosyuMsgs[gemuboId]
	if !exist {
		errmsg := "指定されたIDの募集は存在しません"
		title := arg.commandName
		manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
		return
	}

	delete(manager.bosyuMsgs, gemuboId)
	msg := fmt.Sprintf("ID:%sの募集を削除しました", gemuboId)
	manager.SendNormalMessage(arg.m.ChannelID, "", msg, nil)
}

func onRemovePreset(arg *CommandArg, manager *BotManager) {
	if len(arg.token) < 3 {
		errmsg := "削除するプリセットの名前が指定されていません"
		title := arg.commandName
		manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
		return
	}

	presetName := arg.token[2]
	_, exist := manager.presets[presetName]
	if !exist {
		errmsg := "指定された名前のプリセットは存在しません"
		title := arg.commandName
		manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
		return
	}

	delete(manager.presets, presetName)
	msg := fmt.Sprintf("プリセット:%sを削除しました", presetName)
	manager.SendNormalMessage(arg.m.ChannelID, "", msg, nil)
}

func onRemoveTemplate(arg *CommandArg, manager *BotManager) {
	if len(arg.token) < 3 {
		errmsg := "削除するテンプレートの名前が指定されていません"
		title := arg.commandName
		manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
		return
	}

	templateName := arg.token[2]
	_, exist := manager.templates[templateName]
	if !exist {
		errmsg := "指定された名前のテンプレートは存在しません"
		title := arg.commandName
		manager.SendErrorMessage(arg.m.ChannelID, title, errmsg, nil)
		return
	}

	presetNames := make([]string, 0)
	for _, preset := range manager.presets {
		if preset.Template.Name == templateName {
			presetNames = append(presetNames, preset.Name)
		}
	}

	for _, presetName := range presetNames {
		delete(manager.presets, presetName)
	}

	delete(manager.templates, templateName)

	msg := fmt.Sprintf("テンプレート:%sを削除しました\n", templateName)
	if len(presetNames) > 0 {
		msg += fmt.Sprintf("%sを利用していた以下のプリセットを削除しました\n", templateName)
		for _, presetName := range presetNames {
			msg += fmt.Sprintf("-\t%s\n", presetName)
		}
	}

	manager.SendNormalMessage(arg.m.ChannelID, "", msg, nil)
}

func onHowUseCommand(arg *CommandArg, manager *BotManager) {

	msg := ""
	msg += "**【テンプレートの登録】**\n"
	msg += "\t・まずは、settempl コマンドを利用して募集のテンプレートを登録する\n"
	msg += "\tテンプレート例:\n" + "\t\tゲーム: $GAMES\n" + "\t\t人数: $NUM\n" + "\t\t開始: $START_TIME\n"
	msg += "\n\t($から始まる部分は変数となる。変数は後述の【募集の投稿】で代入する\n"

	msg += "\n**【募集の投稿】**\n"
	msg += "\t・次に、bosyu コマンドを利用して募集を投稿する\n"
	msg += "\t・この時に、変数に値を代入して募集の投稿文を完成させる\n"
	msg += "\n\t変数の代入例:\n" + "\t\tbosyu template=テンプレート名\n" + "\t\t$GAMES=VALORANT\n" + "\t\t$NUM=5\n" + "\t\t$START_TIME=20:00\n"
	msg += "\n\t・$START_TIME変数は特殊であり、時間をhh:mm形式で指定することで開始時刻を設定できる\n"

	msg += "\n**【プリセットの登録】**\n"
	msg += "\t・毎回すべての変数を指定するのは面倒なため、あらかじめ変数の代入値も指定したプリセットをつくることができる\n"
	msg += "\t・setpreset コマンドを利用してプリセットを登録する\n"
	msg += "\n\tコマンド例:\n" + "\t\tsetpreset\ttemplname=テンプレート名\tpresetname=プリセット名\n" + "\t\t$GAMES=VALORANT\n" + "\t\t$NUM=5\n" + "\t\t$START_TIME=20:00\n"
	sendMessage(arg.s, arg.m.ChannelID, msg)
}

func onInvalidCommand(s *discordgo.Session, m *discordgo.MessageCreate, manager *BotManager) {
	msg := "不正なコマンドです。コマンド一覧は「!gemubo help」で確認できます。"
	manager.SendErrorMessage(m.ChannelID, "", msg, nil)
}

func onTestCommand(arg *CommandArg, manager *BotManager) {
	embed := &discordgo.MessageEmbed{
		Title:       "全員集合～！",
		Description: "test",
		Color:       0x00ff00,
	}

	content := ""
	content += manager.BotUserInfo.Mention() + " "
	content += arg.m.Author.Mention() + " \n"
	embed.Description = content

	embeds := make([]*discordgo.MessageEmbed, 0)
	embeds = append(embeds, embed)

	options := &discordgo.MessageSend{
		Reference: &discordgo.MessageReference{
			MessageID: arg.m.ID,
		},
		Embeds: embeds,
	}

	arg.s.ChannelMessageSendComplex(arg.m.ChannelID, options)
}

func paramParse(tokens []string) map[string]string {
	params := make(map[string]string)
	for _, token := range tokens {
		if strings.Contains(token, "=") {
			param := strings.Split(token, "=")
			params[param[0]] = param[1]
		}
	}
	return params
}

func (manager *BotManager) SendNormalMessage(channelId string, title string, msg string, fileds []*discordgo.MessageEmbedField) {
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: msg,
		Color:       0x00ff00,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: manager.BotUserInfo.AvatarURL("20"),
		},
	}

	if fileds != nil {
		embed.Fields = fileds
	}
	_, err := manager.discordSession.ChannelMessageSendEmbed(channelId, embed)
	if err != nil {
		log.Println("Error sending normal embed message\n" + err.Error())
	}
}

func (manager *BotManager) SendErrorMessage(channelId string, title string, msg string, fileds []*discordgo.MessageEmbedField) {
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: msg,
		Color:       0xff0000,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: manager.BotUserInfo.AvatarURL("20"),
		},
	}

	if fileds != nil {
		embed.Fields = fileds
	}
	_, err := manager.discordSession.ChannelMessageSendEmbed(channelId, embed)
	if err != nil {
		log.Println("Error sending error embed message\n" + err.Error())
	}
}

func isVariable(token string) bool {
	return strings.HasPrefix(token, "$")
}

func isSameMessage(msg *discordgo.Message, gemuboID string) bool {
	tokens := strings.Split(msg.Content, "\n")
	if len(tokens) < 2 {
		return false
	}

	gemuboIDTokens := strings.Split(tokens[1], ":")
	if len(gemuboIDTokens) < 2 {
		return false
	}

	return gemuboIDTokens[1] == gemuboID
}

func isSameEmbed(emb *discordgo.MessageEmbed, gemuboID string) bool {
	tokens := strings.Split(emb.Description, "\n")
	if len(tokens) < 1 {
		return false
	}

	gemuboIDTokens := strings.Split(tokens[0], ":")
	if len(gemuboIDTokens) < 2 {
		return false
	}

	return gemuboIDTokens[1] == gemuboID
}
