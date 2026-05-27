package handler

import (
	"net/http"
	"strconv"

	"github.com/Hana-ame/api-pack/shijima/model"
	tools "github.com/Hana-ame/api-pack/utils/utils"
	"github.com/gin-gonic/gin"
)

func (h *Handler) V2Get(c *gin.Context) {
	bid := tools.Atoi(c.Query("bid"), 0)
	tid := tools.Atoi(c.Query("tid"), 0)
	pn := tools.Atoi(c.Query("pn"), 0)

	if tid == 0 && bid != 0 {
		board, err := h.Repo.V2GetBoard(bid, pn)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, board)
	} else if tid != 0 {
		thread, err := h.Repo.ThreadWithReplies(tid, pn)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, thread)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "set tid or bid"})
	}
}

func (h *Handler) V2Post(c *gin.Context) {
	bid := tools.Atoi(c.Query("bid"), 0)
	tid := tools.Atoi(c.Query("tid"), 0)

	var t model.Thread
	if err := c.BindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	t.ReplyTo = tools.Or(t.ReplyTo, uint(tid))
	if t.ReplyTo == 0 && bid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "set tid or bid"})
		return
	}

	t.ID = c.GetString("id")
	t.Country = c.GetHeader("Cf-Ipcountry")
	t.IP = c.GetHeader("X-Forwarded-For")

	lastID, err := h.Repo.V2PostThread(&t, bid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	t.No = uint(lastID)
	c.AbortWithStatus(http.StatusOK)
}

func (h *Handler) V2Delete(c *gin.Context) {
	id := c.GetString("id")
	ip := c.GetHeader("X-Forwarded-For")
	no := tools.Atoi(c.Query("no"), 0)

	if err := h.Repo.V2DeleteThread(no, id, ip); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "ip": ip, "deleted": no})
}

// ---- Reactions ----

func (h *Handler) V2GetReactions(c *gin.Context) {
	tid, err := strconv.Atoi(c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tid"})
		return
	}
	counts, err := h.Repo.ReactionGet(tid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reactions": counts, "my_reaction": ""})
}

func (h *Handler) V2SetReaction(c *gin.Context) {
	tid, err := strconv.Atoi(c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tid"})
		return
	}
	reaction := string(tools.Match(c.GetRawData()).Result())
	if err := h.Repo.ReactionSet(tid, reaction); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if reaction == "🎲" {
		h.Repo.V2PostThread(&model.Thread{
			ID:      c.GetString("id"),
			ReplyTo: uint(tid),
			Content: "auto roll [1,100]\n@rd 1d100",
		}, 0)
	}
	c.JSON(http.StatusOK, gin.H{"message": reaction})
}

// ---- New Reactions ----

func (h *Handler) MarkNewReaction(tid int) { h.newReactions.put(tid) }

func (h *Handler) GetNewReactionsHandler(c *gin.Context) {
	if id := c.Query("delete"); id != "" {
		h.newReactions.delete(tools.Atoi(id, 0))
		c.JSON(http.StatusOK, gin.H{"deleted": id})
		return
	}

	ids := h.newReactions.getOrder()
	if len(ids) == 0 {
		c.JSON(http.StatusOK, []*model.Thread{})
		return
	}

	results := make([]*model.BoardThread, 0, len(ids))
	for _, tid := range ids {
		thread, err := h.Repo.V2GetThread(tid)
		if err != nil {
			continue
		}
		replies, _ := h.Repo.V2GetRepliesPreview(int(thread.No))
		results = append(results, &model.BoardThread{Thread: *thread, Replies: replies})
	}
	c.JSON(http.StatusOK, results)
}
