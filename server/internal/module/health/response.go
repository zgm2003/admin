package health

type Status struct {
	Status string `json:"status"`
}

type Readiness struct {
	PostgreSQL string `json:"postgresql"`
	Redis      string `json:"redis"`
}
