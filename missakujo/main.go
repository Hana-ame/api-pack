package missakujo

import (
	"github.com/gofiber/fiber/v2"
)

type DelReqCtx struct {
	Host  string `json:"host"`
	User  string `json:"user"`
	Token string `json:"token"`
	Since string `json:"since"`
	Until string `json:"until"`

	RenoteLessThan int `json:"renoteLessThan"`

	TimeOffset int `json:"timeOffset"`
}

const timeForm = "2006-01-02 15:04:05"

func App() *fiber.App {
	return nil
}
