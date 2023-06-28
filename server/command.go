package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

const (
	listHeaderMessage = " Список задач:\n\n"
	MyFlag            = "my"
	InFlag            = "in"
	OutFlag           = "out"
)

func getHelp() string {
	return `Доступные команды:

add [сообщение]
	Добавить задачу.

	пример: /todo add Не забудь сделать что-нибудь крутое

list
	Вывести список ваших задач.

list [listName]
	Вывести список задач конкретной категории

	пример: /todo list in
	пример: /todo list out
	пример (same as /todo list): /todo list my

pop
	Убрать задачу с конца очереди задач.

send [user] [message]
	Послать задачу указанному пользователю

	пример: /todo send @пользователь Не забудь сделать что-нибудь полезное

settings summary [on, off]
	Включить/выключить напоминания о предстоящих задачах пользователю

	пример: /todo settings summary on

settings allow_incoming_task_requests [on, off]
	Разрешить другим пользователям пересылать вам задачи?

	пример: /todo settings allow_incoming_task_requests on


help
	Отобразить эту помощь.
`
}

func getSummarySetting(flag bool) string {
	if flag {
		return "Reminder setting is set to `on`. **You will receive daily reminders.**"
	}
	return "Reminder setting is set to `off`. **You will not receive daily reminders.**"
}
func getAllowIncomingTaskRequestsSetting(flag bool) string {
	if flag {
		return "Allow incoming task requests setting is set to `on`. **Other users can send you task request that you can accept/decline.**"
	}
	return "Allow incoming task requests setting is set to `off`. **Other users cannot send you task request. They will see a message saying you don't accept Todo requests.**"
}

func getAllSettings(summaryFlag, blockIncomingFlag bool) string {
	return fmt.Sprintf(`Current Settings:

%s
%s
	`, getSummarySetting(summaryFlag), getAllowIncomingTaskRequestsSetting(blockIncomingFlag))
}

func getCommand() *model.Command {
	return &model.Command{
		Trigger:          "todo",
		DisplayName:      "Робот задач",
		Description:      "Работа со списков ваших задач.",
		AutoComplete:     true,
		AutoCompleteDesc: "Доступные команды: add, list, pop, send, help",
		AutoCompleteHint: "[command]",
		AutocompleteData: getAutocompleteData(),
	}
}

func (p *Plugin) postCommandResponse(args *model.CommandArgs, text string) {
	post := &model.Post{
		UserId:    p.BotUserID,
		ChannelId: args.ChannelId,
		Message:   text,
	}
	_ = p.API.SendEphemeralPost(args.UserId, post)
}

// ExecuteCommand executes a given command and returns a command response.
func (p *Plugin) ExecuteCommand(_ *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	spaceRegExp := regexp.MustCompile(`\s+`)
	trimmedArgs := spaceRegExp.ReplaceAllString(strings.TrimSpace(args.Command), " ")
	stringArgs := strings.Split(trimmedArgs, " ")
	lengthOfArgs := len(stringArgs)
	restOfArgs := []string{}

	var handler func([]string, *model.CommandArgs) (bool, error)
	if lengthOfArgs == 1 {
		handler = p.runListCommand
		p.trackCommand(args.UserId, "")
	} else {
		command := stringArgs[1]
		if lengthOfArgs > 2 {
			restOfArgs = stringArgs[2:]
		}
		switch command {
		case "add":
			handler = p.runAddCommand
		case "list":
			handler = p.runListCommand
		case "pop":
			handler = p.runPopCommand
		case "send":
			handler = p.runSendCommand
		case "settings":
			handler = p.runSettingsCommand
		default:
			if command == "help" {
				p.trackCommand(args.UserId, command)
			} else {
				p.trackCommand(args.UserId, "not found")
			}
			p.postCommandResponse(args, getHelp())
			return &model.CommandResponse{}, nil
		}
		p.trackCommand(args.UserId, command)
	}
	isUserError, err := handler(restOfArgs, args)
	if err != nil {
		if isUserError {
			p.postCommandResponse(args, fmt.Sprintf("__Error: %s.__\n\nRun `/todo help` для отображения инструкций как пользоваться командами.", err.Error()))
		} else {
			p.API.LogError(err.Error())
			p.postCommandResponse(args, "Произошла ошибка. Обратитесь за помощью к супер программисту.")
		}
	}

	return &model.CommandResponse{}, nil
}

func (p *Plugin) runSendCommand(args []string, extra *model.CommandArgs) (bool, error) {
	if len(args) < 2 {
		p.postCommandResponse(extra, "Вы должны указать пользователя и сообщение.\n"+getHelp())
		return false, nil
	}

	userName := args[0]
	if args[0][0] == '@' {
		userName = args[0][1:]
	}
	receiver, appErr := p.API.GetUserByUsername(userName)
	if appErr != nil {
		p.postCommandResponse(extra, "Пожалуйста, укажите пользователя правильно.\n"+getHelp())
		return false, nil
	}

	if receiver.Id == extra.UserId {
		return p.runAddCommand(args[1:], extra)
	}

	receiverAllowIncomingTaskRequestsPreference, err := p.getAllowIncomingTaskRequestsPreference(receiver.Id)
	if err != nil {
		p.API.LogError("Ошибка при получении разрешения на запрос входящей задачи, err=", err)
		receiverAllowIncomingTaskRequestsPreference = true
	}
	if !receiverAllowIncomingTaskRequestsPreference {
		p.postCommandResponse(extra, fmt.Sprintf("Пользователем @%s задача была отклонена.", userName))
		return false, nil
	}

	message := strings.Join(args[1:], " ")

	receiverIssueID, err := p.listManager.SendIssue(extra.UserId, receiver.Id, message, "", "")
	if err != nil {
		return false, err
	}

	p.trackSendIssue(extra.UserId, sourceCommand, false)

	p.sendRefreshEvent(extra.UserId, []string{OutListKey})
	p.sendRefreshEvent(receiver.Id, []string{InListKey})

	responseMessage := fmt.Sprintf("Задача была отправлена @%s.", userName)

	senderName := p.listManager.GetUserName(extra.UserId)

	receiverMessage := fmt.Sprintf("Вы получили новую задачу от @%s", senderName)

	p.PostBotCustomDM(receiver.Id, receiverMessage, message, receiverIssueID)
	p.postCommandResponse(extra, responseMessage)
	return false, nil
}

func (p *Plugin) runAddCommand(args []string, extra *model.CommandArgs) (bool, error) {
	message := strings.Join(args, " ")

	if message == "" {
		p.postCommandResponse(extra, "Пожалуйста, добавте задачу.")
		return false, nil
	}

	newIssue, err := p.listManager.AddIssue(extra.UserId, message, "", "")
	if err != nil {
		return false, err
	}

	p.trackAddIssue(extra.UserId, sourceCommand, false)

	p.sendRefreshEvent(extra.UserId, []string{MyListKey})

	responseMessage := "Задача добавлена."

	issues, err := p.listManager.GetIssueList(extra.UserId, MyListKey)
	if err != nil {
		p.API.LogError(err.Error())
		p.postCommandResponse(extra, responseMessage)
		return false, nil
	}

	// It's possible that database replication delay has resulted in the issue
	// list not containing the newly-added issue, so we check for that and
	// append the issue manually if necessary.
	var issueIncluded bool
	for _, issue := range issues {
		if newIssue.ID == issue.ID {
			issueIncluded = true
			break
		}
	}
	if !issueIncluded {
		issues = append(issues, &ExtendedIssue{
			Issue: *newIssue,
		})
	}

	responseMessage += listHeaderMessage
	responseMessage += issuesListToString(issues)
	p.postCommandResponse(extra, responseMessage)

	return false, nil
}

func (p *Plugin) runListCommand(args []string, extra *model.CommandArgs) (bool, error) {
	listID := MyListKey
	responseMessage := "Список задач:\n\n"

	if len(args) > 0 {
		switch args[0] {
		case MyFlag:
		case InFlag:
			listID = InListKey
			responseMessage = "Список полученных задач:\n\n"
		case OutFlag:
			listID = OutListKey
			responseMessage = "Список отправленных задач:\n\n"
		default:
			p.postCommandResponse(extra, getHelp())
			return true, nil
		}
	}

	issues, err := p.listManager.GetIssueList(extra.UserId, listID)
	if err != nil {
		return false, err
	}

	p.sendRefreshEvent(extra.UserId, []string{MyListKey, OutListKey, InListKey})

	responseMessage += issuesListToString(issues)
	p.postCommandResponse(extra, responseMessage)

	return false, nil
}

func (p *Plugin) runPopCommand(_ []string, extra *model.CommandArgs) (bool, error) {
	issue, foreignID, err := p.listManager.PopIssue(extra.UserId)
	if err != nil {
		if err.Error() == "cannot find issue" {
			p.postCommandResponse(extra, "Здесь нет задач для выполнения.")
			return false, nil
		}
		return false, err
	}

	userName := p.listManager.GetUserName(extra.UserId)

	if foreignID != "" {
		p.sendRefreshEvent(foreignID, []string{OutListKey})

		message := fmt.Sprintf("Пользователь @%s направил вам задачу: %s", userName, issue.Message)
		p.PostBotDM(foreignID, message)
	}

	p.sendRefreshEvent(extra.UserId, []string{MyListKey})

	responseMessage := "Удалена последняя задача из списка."

	replyMessage := fmt.Sprintf("Пользователь @%s взял задачу прикреплённую к каналу", userName)
	p.postReplyIfNeeded(issue.PostID, replyMessage, issue.Message)

	issues, err := p.listManager.GetIssueList(extra.UserId, MyListKey)
	if err != nil {
		p.API.LogError(err.Error())
		p.postCommandResponse(extra, responseMessage)
		return false, nil
	}

	responseMessage += listHeaderMessage
	responseMessage += issuesListToString(issues)
	p.postCommandResponse(extra, responseMessage)

	return false, nil
}

func (p *Plugin) runSettingsCommand(args []string, extra *model.CommandArgs) (bool, error) {
	const (
		on  = "on"
		off = "off"
	)
	if len(args) < 1 {
		currentSummarySetting := p.getReminderPreference(extra.UserId)
		currentAllowIncomingTaskRequestsSetting, err := p.getAllowIncomingTaskRequestsPreference(extra.UserId)
		if err != nil {
			p.API.LogError("Ошибка при получении разрешения на запрос входящей задачи, err=", err)
			currentAllowIncomingTaskRequestsSetting = true
		}
		p.postCommandResponse(extra, getAllSettings(currentSummarySetting, currentAllowIncomingTaskRequestsSetting))
		return false, nil
	}

	switch args[0] {
	case "summary":
		if len(args) < 2 {
			currentSummarySetting := p.getReminderPreference(extra.UserId)
			p.postCommandResponse(extra, getSummarySetting(currentSummarySetting))
			return false, nil
		}
		if len(args) > 2 {
			return true, errors.New("слишком много аргументов")
		}
		var responseMessage string
		var err error

		switch args[1] {
		case on:
			err = p.saveReminderPreference(extra.UserId, true)
			responseMessage = "вы включили ежедневные напоминания по предстоящим задачам."
		case off:
			err = p.saveReminderPreference(extra.UserId, false)
			responseMessage = "вы выключили ежедневные напоминания по предстоящим задачам."
		default:
			responseMessage = "неверно переданные параметры для \"settings summary\". Должно быть `on` или `off`"
			return true, errors.New(responseMessage)
		}

		if err != nil {
			responseMessage = "ошибка при сохранении настройки напоминания"
			p.API.LogDebug("runSettingsCommand: ошибка при сохранении настройки напоминания", "error", err.Error())
			return false, errors.New(responseMessage)
		}

		p.postCommandResponse(extra, responseMessage)

	case "allow_incoming_task_requests":
		if len(args) < 2 {
			currentAllowIncomingTaskRequestsSetting, err := p.getAllowIncomingTaskRequestsPreference(extra.UserId)
			if err != nil {
				p.API.LogError("не удалось проанализировать параметр разрешения входящих запросов задач, err=", err.Error())
				currentAllowIncomingTaskRequestsSetting = true
			}
			p.postCommandResponse(extra, getAllowIncomingTaskRequestsSetting(currentAllowIncomingTaskRequestsSetting))
			return false, nil
		}
		if len(args) > 2 {
			return true, errors.New("слишком много параметров")
		}
		var responseMessage string
		var err error

		switch args[1] {
		case on:
			err = p.saveAllowIncomingTaskRequestsPreference(extra.UserId, true)
			responseMessage = "Другие пользователи могут отправить вам задания для принятия/отклонения"
		case off:
			err = p.saveAllowIncomingTaskRequestsPreference(extra.UserId, false)
			responseMessage = "Другие пользователи не могут отправить вам запрос на задачу. Они увидят сообщение о том, что вы заблокировали входящие запросы."
		default:
			responseMessage = "неверный ввод, допустимые значения для \"settings allow_incoming_task_requests\" должно быть `on` или `off`"
			return true, errors.New(responseMessage)
		}

		if err != nil {
			responseMessage = "ошибка сохранения block_incoming"
			p.API.LogDebug("runSettingsCommand: ошибка сохранения block_incoming", "error", err.Error())
			return false, errors.New(responseMessage)
		}

		p.postCommandResponse(extra, responseMessage)
	default:
		return true, fmt.Errorf("настройка `%s` не допустима", args[0])
	}
	return false, nil
}

func getAutocompleteData() *model.AutocompleteData {
	todo := model.NewAutocompleteData("todo", "[command]", "Доступные команды: list, add, pop, send, settings, help")

	add := model.NewAutocompleteData("add", "[message]", "Добавление задачи")
	add.AddTextArgument("Например. будь офигенным", "[message]", "")
	todo.AddCommand(add)

	list := model.NewAutocompleteData("list", "[name]", "Список ваших задач")
	items := []model.AutocompleteListItem{{
		HelpText: "Полученные задачи",
		Hint:     "(optional)",
		Item:     "in",
	}, {
		HelpText: "Пересланные задачи",
		Hint:     "(optional)",
		Item:     "out",
	}}
	list.AddStaticListArgument("Список ваших задач", false, items)
	todo.AddCommand(list)

	pop := model.NewAutocompleteData("pop", "", "Удаляет последнюю задачу из списка")
	todo.AddCommand(pop)

	send := model.NewAutocompleteData("send", "[user] [todo]", "Посылает задачу указанному пользователю")
	send.AddTextArgument("Whom to send", "[@awesomePerson]", "")
	send.AddTextArgument("Todo message", "[message]", "")
	todo.AddCommand(send)

	settings := model.NewAutocompleteData("settings", "[setting] [on] [off]", "Включает настройку пользователя")
	summary := model.NewAutocompleteData("summary", "[on] [off]", "Включает настройку ежедневных напоминаний")
	summaryOn := model.NewAutocompleteData("on", "", "включить ежедневные напоминания")
	summaryOff := model.NewAutocompleteData("off", "", "выключить ежедневные напоминания")
	summary.AddCommand(summaryOn)
	summary.AddCommand(summaryOff)

	allowIncomingTask := model.NewAutocompleteData("allow_incoming_task_requests", "[on] [off]", "Разрешить другим пользователям отправлять вам задание, чтобы вы могли его принять или отклонить?")
	allowIncomingTaskOn := model.NewAutocompleteData("on", "", "Разрешить другим пользователям отправлять вам задачи")
	allowIncomingTaskOff := model.NewAutocompleteData("off", "", "Заблокировать от отправки вам задач от других пользователей. Они увидят сообщение о том, что вы не принимаете запросы задач.")
	allowIncomingTask.AddCommand(allowIncomingTaskOn)
	allowIncomingTask.AddCommand(allowIncomingTaskOff)

	settings.AddCommand(summary)
	settings.AddCommand(allowIncomingTask)
	todo.AddCommand(settings)

	help := model.NewAutocompleteData("help", "", "Display usage")
	todo.AddCommand(help)
	return todo
}
