package shijima

import (
	"strconv"
	"sync"

	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-multierror"
)

var new50 = *tools.NewLRUCache[int, bool](50)

func deleteNewReaction(c *gin.Context) {
	// 写在get里面
	return
}

func updateNewReaction(c *gin.Context) {
	tidStr := c.Param("tid")
	tid, err := strconv.Atoi(tidStr)
	if err != nil {
		return
	}
	new50.Put(tid, true)

	return
}

func getNewReactions(c *gin.Context) {
	if id := c.Query("delete"); id != "" {
		new50.Delete(tools.Atoi(id, 0))
		c.JSON(200, gin.H{"status": "moved " + id})
		return
	}

	list := new50.GetOrder()

	n := len(list)
	threads := make([]*Thread, n)
	errs1 := make([]error, n)
	errs2 := make([]error, n)

	var wg sync.WaitGroup
	wg.Add(n) // [1,2,6](@ref)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			threads[i], errs1[i] = getThreadByNo(list[i])
			threads[i].List, errs2[i] = getRepliesPreview(int(threads[i].No))
		}(i)
	}
	wg.Wait()

	var merr *multierror.Error
	merr = multierror.Append(merr, errs1...)
	merr = multierror.Append(merr, errs2...)
	if tools.AbortWithError(c, 500, merr.ErrorOrNil()) {
		return
	}

	c.JSON(200, threads)
}
