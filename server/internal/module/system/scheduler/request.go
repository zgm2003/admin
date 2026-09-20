package scheduler

import (
	"encoding/json"
	"errors"
	"strings"
)

type scheduleRequest struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	TaskType       string          `json:"taskType"`
	CronExpression string          `json:"cronExpression"`
	Timezone       string          `json:"timezone"`
	Params         json.RawMessage `json:"params"`
	IsEnabled      bool            `json:"isEnabled"`
}

func (r scheduleRequest) input() (ScheduleInput, error) {
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	r.TaskType = strings.TrimSpace(r.TaskType)
	r.CronExpression = strings.TrimSpace(r.CronExpression)
	r.Timezone = strings.TrimSpace(r.Timezone)
	if len(r.Params) == 0 {
		r.Params = json.RawMessage(`{}`)
	}
	if r.Name == "" || len(r.Name) > 128 || len(r.Description) > 512 || r.TaskType == "" || r.CronExpression == "" || r.Timezone == "" {
		return ScheduleInput{}, errors.New("scheduler schedule fields are invalid")
	}
	return ScheduleInput{Name: r.Name, Description: r.Description, TaskType: r.TaskType, CronExpression: r.CronExpression, Timezone: r.Timezone, Params: r.Params, Enabled: r.IsEnabled}, nil
}

type statusRequest struct {
	IsEnabled bool `json:"isEnabled"`
}
