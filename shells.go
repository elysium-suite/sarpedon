package main

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

func refreshShell(teamId string, imageName string, image *imageShell) {
	fmt.Println("Refreshing shell!")
	image.StdinRead, image.StdinWrite = io.Pipe()
	image.StdoutRead, image.StdoutWrite = io.Pipe()
	sarpShells[teamId] = make(map[string]*imageShell)
	sarpShells[teamId][imageName] = image
}

func initShell(c *gin.Context) (*imageShell, error) {
	teamId := c.Param("id")
	if !validateTeam(teamId) {
		err := errors.New("Invalid team id: " + teamId)
		return &imageShell{}, err
	}
	imageName := c.Param("image")
	if !validateImage(imageName) {
		err := errors.New("Invalid image name: " + imageName)
		return &imageShell{}, err
	}

	fmt.Println("FETCHING FOR teamid", teamId, "imageName", imageName)
	image := &imageShell{}
	if img, ok := sarpShells[teamId][imageName]; ok {
		fmt.Println("found image alread made", img)
		image = img
	} else {
		fmt.Println("making new img", img)
		refreshShell(teamId, imageName, image)
	}
	return image, nil
}

func shellServerInput(c *gin.Context) {
	image, err := initShell(c)
	if err != nil {
		errorOut(c, err)
		return
	}
	cn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Print("upgrade:", err)
		return
	}
	defer cn.Close()
	for {
		_, message, err := cn.ReadMessage()
		if err != nil {
			fmt.Println("read 1:", err)
			break
		}
		fmt.Printf("writing to StdinWrite: %s", message)
		fmt.Fprintf(image.StdinWrite, string(message)+"\n")
		if string(message) == "exit" {
			break
		}
		if err != nil {
			fmt.Println("write:", err)
			break
		}
	}
	image.Active = false
	image.Waiting = false
	fmt.Println("sending exit 1")
	fmt.Fprintf(image.StdinWrite, "exit")
}

func shellServerOutput(c *gin.Context) {
	image, err := initShell(c)
	timeOfCreation := time.Now()
	if err != nil {
		errorOut(c, err)
		return
	}
	cn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Print("upgrade:", err)
		return
	}
	defer cn.Close()
	for {
		buffer := make([]byte, 1024)
		n, _ := image.StdoutRead.Read(buffer)
		fmt.Println(timeOfCreation, "GOT INPUT FROM CLIENT YO", string(buffer))
		err = cn.WriteMessage(1, buffer[:n])
		image.Waiting = false
		if err != nil {
			fmt.Println("write:", err)
			break
		}
		if string(buffer[:n]) == "exit" {
			break
		}
	}
	image.Waiting = false
}

func shellClientInput(c *gin.Context) {
	image, err := initShell(c)
	if err != nil {
		errorOut(c, err)
		return
	}
	cn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Print("upgrade:", err)
		return
	}
	defer cn.Close()
	for {
		buffer := make([]byte, 1024)
		n, _ := image.StdinRead.Read(buffer)
		fmt.Println("sending STDIN!!!!", string(buffer))
		err = cn.WriteMessage(1, buffer[:n])
		if err != nil {
			fmt.Println("write:", err)
			break
		}
		if string(buffer[:n]) == "exit" {
			break
		}
	}
	fmt.Println("sending exit 2")
	err = cn.WriteMessage(1, []byte("exit"))
}

func shellClientOutput(c *gin.Context) {
	image, err := initShell(c)
	if err != nil {
		errorOut(c, err)
		return
	}
	cn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Print("upgrade:", err)
		return
	}
	defer cn.Close()
	prevError := false
	for {
		_, message, err := cn.ReadMessage()
		if err != nil {
			fmt.Println("read 3:", err)
			if !prevError {
				teamId := c.Param("id")
				imageName := c.Param("image")
				fmt.Println("REFRESHING DUE TO READ ERROR!")
				refreshShell(teamId, imageName, image)
				prevError = true
				continue
			}
			break
		}
		fmt.Printf("writing to StdoutWrite: %s", message)
		fmt.Fprintf(image.StdoutWrite, string(message))
		if string(message) == "exit" {
			break
		}
		if err != nil {
			fmt.Println("write:", err)
			break
		}
	}
	fmt.Fprintf(image.StdoutWrite, "exit")
}
