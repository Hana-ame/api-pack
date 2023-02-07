package missakujo

type DelReqCtx struct {
	host  string `json:"host"`
	user  string `json:"user"`
	token string `json:"token"`
	since string `json:"since"`
	until string `json:"until"`

	renoteLessThan int `json:"renoteLessThan"`
}

const timeForm = "2006-01-02 15:04:05"

func HandleDelete(delReqCtx *DelReqCtx) {

}
