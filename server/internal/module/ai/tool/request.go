package tool

import "fmt"

type statusRequest struct {
	Status *int16 `json:"status"`
}

func (r statusRequest) input() (Status, error) {
	if r.Status == nil {
		return 0, fmt.Errorf("status is required")
	}
	status := Status(*r.Status)
	if !status.Valid() {
		return 0, fmt.Errorf("status is invalid")
	}
	return status, nil
}
