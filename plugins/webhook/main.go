package main

import (
	"fmt"
	"net/http"
	"os/exec"

	"github.com/gin-gonic/gin"
)

const BaseRoute = "/api/webhook"
const Port = ":8886"

func main() {
	r := gin.Default()

	r.POST(BaseRoute, DealRequestToExecShell)

	err := r.Run(Port)

	if err != nil {
		return
	}
}

func DealRequestToExecShell(c *gin.Context) {
	// 定义接收数据的结构体
	type Command struct {
		Input string `form:"command" json:"command" binding:"required"`
	}
	var json Command
	// 将request的body中的数据，自动按照json格式解析到结构体
	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cmd := exec.Command("/bin/bash", "-c", json.Input)

	output, err := cmd.Output()
	if err != nil {
		c.JSON(500, gin.H{"msg": fmt.Sprintf("Execute Command:%s failed with error:%s", json.Input, err.Error())})
		return
	}

	c.JSON(200, gin.H{"msg": fmt.Sprintf("%s ➡️ %s", json.Input, string(output))})
}
