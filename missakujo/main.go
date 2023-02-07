package missakujo

type DelReqCtx struct {
	Host  string `json:"host"`
	User  string `json:"user"`
	Token string `json:"token"`
	Since string `json:"since"`
	Until string `json:"until"`

	RenoteLessThan int `json:"renoteLessThan"`
}

const timeForm = "2006-01-02 15:04:05"

func HandleDelete(delReqCtx *DelReqCtx) {

}
