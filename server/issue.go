package main

import (
	"fmt"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
)

// Issue represents a Todo issue
type Issue struct {
	ID          string `json:"id"`
	Message     string `json:"message"`
	Description string `json:"description,omitempty"`
	CreateAt    int64  `json:"create_at"`
	PostID      string `json:"post_id"`
}

// ExtendedIssue extends the information on Issue to be used on the front-end
type ExtendedIssue struct {
	Issue
	ForeignUser     string `json:"user"`
	ForeignList     string `json:"list"`
	ForeignPosition int    `json:"position"`
}

func newIssue(message string, description, postID string) *Issue {
	return &Issue{
		ID:          model.NewId(),
		CreateAt:    model.GetMillis(),
		Message:     message,
		Description: description,
		PostID:      postID,
	}
}

func issuesListToString(issues []*ExtendedIssue) string {
	if len(issues) == 0 {
		return "Делать нечего!"
	}

	str := "\n\n"

	MONTHS := [12]string{"Янв", "Фев", "Мар", "Апр", "Май", "Июн", "Июл", "Авг", "Сен", "Окт", "Ноя", "Дек"}

	for _, issue := range issues {
		createAt := time.Unix(issue.CreateAt/1000, 0)

		year := createAt.Year()
		month := MONTHS[int(createAt.Month())]
		day := createAt.Day()
		hours := fmt.Sprintf("0%d", createAt.Hour())
		minutes := fmt.Sprintf("0%d", createAt.Minute())
		seconds := fmt.Sprintf("0%d", createAt.Second())

		formattedTime := fmt.Sprintf("%d %s %d %s:%s:%s", day, month, year, string(hours[len(hours)-2:]), string(minutes[len(minutes)-2:]), string(minutes[len(seconds)-2:]))

		str += fmt.Sprintf("* %s\n  * (%s)\n", issue.Message, formattedTime)
	}

	return str
}
