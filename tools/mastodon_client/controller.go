// 能用，但是家里云的upload限制太死了，用不了。

package mastodonclient

import (
	"net/http"

	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

func Upload(client *Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, url, err := client.Upload(c.Request.Body)
		if tools.AbortWithError(c, http.StatusBadGateway, err) {
			return
		}

		err = client.PostStatus("", "", []string{id}, true, "", "private", nil, "zh")
		if tools.AbortWithError(c, http.StatusBadGateway, err) {
			return
		}

		c.Redirect(http.StatusFound, url)
	}
}

// 250712 将mastodon作为图床
// api.PUT("/mastodon/upload", mastodonclient.Upload(&mastodonclient.Client{
// 	Host:          os.Getenv("MASTODON_HOST"),
// 	Cookie:        os.Getenv("MASTODON_COOKIE"),
// 	Authorization: os.Getenv("MASTODON_AUTHORIZATION"),
// }))
