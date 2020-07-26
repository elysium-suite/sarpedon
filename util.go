package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

func errorOut(c *gin.Context, err error) {
	fmt.Println("ERROR:", err)
	c.JSON(400, gin.H{"error": "Invalid request."})
	c.Abort()
}

func errorOutGraceful(c *gin.Context, err error) {
	fmt.Println("ERROR:", err)
	c.Redirect(http.StatusSeeOther, "/")
	c.Abort()
}
