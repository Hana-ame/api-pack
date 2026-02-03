package main

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// --- Types & Models ---

type StatusType string

const (
	StatusEstimating StatusType = "估计cost中"
	StatusWaiting    StatusType = "等待上传"
	StatusCompleted  StatusType = "已完成"
	StatusClosed     StatusType = "已经关闭"
)

type GalleryItem struct {
	ID     int        `json:"id"`
	G      string     `json:"g"`
	Title  string     `json:"title"`
	Cover  string     `json:"cover"`
	Status StatusType `json:"status"`

	// Submitter info
	SubmitterID      string    `json:"submitter_id"`
	SubmitterBalance float64   `json:"submitter_balance"`
	SubmitTime       time.Time `json:"submit_time"`
	SubmitterMessage string    `json:"submitter_message,omitempty"`

	// Cost
	EstimatedCost float64 `json:"estimated_cost"`

	// Uploader info
	UploaderID      string    `json:"uploader_id,omitempty"`
	UploaderBalance float64   `json:"uploader_balance,omitempty"`
	DownloadURL     string    `json:"download_url,omitempty"`
	UploadTime      time.Time `json:"upload_time,omitempty"`
	UploadMessage   string    `json:"upload_message,omitempty"`

	// Voting
	SubLikes    int `json:"sub_likes"`
	SubDislikes int `json:"sub_dislikes"`
	GLikes      int `json:"g_likes"`
	GDislikes   int `json:"g_dislikes"`
}

// --- In-Memory Database ---

var (
	items  = []GalleryItem{}
	nextID = 1
	mu     sync.RWMutex
)

func main() {

	r := gin.Default()

	// CORS Middleware (Allow frontend access)
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// --- Routes ---

	// 1. GET /new - Generate unique User ID
	r.GET("/new", func(c *gin.Context) {
		newID := uuid.New().String()[:8]
		c.JSON(http.StatusOK, gin.H{"userId": newID})
	})

	// 2. GET /items - Fetch based on Tab
	r.GET("/items", func(c *gin.Context) {
		tab := c.Query("tab")
		userID := c.Query("userId")

		mu.RLock()
		defer mu.RUnlock()

		filtered := []GalleryItem{}

		switch tab {
		case "recent_submit":
			filtered = append(filtered, items...)
			sort.Slice(filtered, func(i, j int) bool {
				return filtered[i].SubmitTime.After(filtered[j].SubmitTime)
			})

		case "recent_upload":
			for _, item := range items {
				if item.Status == StatusCompleted {
					filtered = append(filtered, item)
				}
			}
			sort.Slice(filtered, func(i, j int) bool {
				return filtered[i].UploadTime.After(filtered[j].UploadTime)
			})

		case "related":
			for _, item := range items {
				if item.SubmitterID == userID || item.UploaderID == userID {
					filtered = append(filtered, item)
				}
			}

		default:
			filtered = items
		}

		c.JSON(http.StatusOK, filtered)
	})

	// 3. POST /submit - Create new request
	r.POST("/submit", func(c *gin.Context) {
		var input GalleryItem
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		mu.Lock()
		input.ID = nextID
		nextID++
		input.SubmitTime = time.Now()
		input.Status = StatusEstimating
		items = append(items, input)
		mu.Unlock()

		c.JSON(http.StatusCreated, input)
	})

	// 4. POST /items/:id/cost - Set cost by admin
	r.POST("/items/:id/cost", func(c *gin.Context) {
		idStr := c.Param("id")
		var body struct {
			Cost float64 `json:"cost"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cost"})
			return
		}

		mu.Lock()
		defer mu.Unlock()
		for i, item := range items {
			if fmt.Sprintf("%d", item.ID) == idStr {
				items[i].EstimatedCost = body.Cost
				items[i].Status = StatusWaiting
				c.JSON(http.StatusOK, items[i])
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
	})

	// 5. POST /items/:id/submission/vote
	r.POST("/items/:id/submission/vote", func(c *gin.Context) {
		idStr := c.Param("id")
		var body struct {
			Type string `json:"type"` // "like" or "dislike"
		}
		c.ShouldBindJSON(&body)

		mu.Lock()
		defer mu.Unlock()
		for i, item := range items {
			if fmt.Sprintf("%d", item.ID) == idStr {
				if body.Type == "like" {
					items[i].SubLikes++
				} else {
					items[i].SubDislikes++
				}
				c.JSON(http.StatusOK, items[i])
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
	})

	r.Run("0.0.0.0:8888")
	// r.Run("127.26.1.31:8080")
}
