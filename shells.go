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

func refreshShell(teamID, imageName string, image *imageShell) {
	fmt.Println("Refreshing shell!")
	image.StdinRead, image.StdinWrite = io.Pipe()
	image.StdoutRead, image.StdoutWrite = io.Pipe()
	sarpShells[teamID] = make(map[string]*imageShell)
	sarpShells[teamID][imageName] = image
}

func initShell(c *gin.Context) (*imageShell, error) {
	teamID := c.Param("id")
	if !validateTeam(teamID) {
		err := errors.New("Invalid team id: " + teamID)
		return &imageShell{}, err
	}
	imageName := c.Param("image")
	if !validateImage(imageName) {
		err := errors.New("Invalid image name: " + imageName)
		return &imageShell{}, err
	}

	image := &imageShell{}
	if img, ok := sarpShells[teamID][imageName]; ok {
		image = img
	} else {
		refreshShell(teamID, imageName, image)
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
	fmt.Println("SETTING image.ACTIVE TO TRUE")
	image.Active = true
	image.Waiting = true
	for {
		if !image.Active {
			break
		}
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
	fmt.Println("SETTING ACTIVE TO FALSE")
	image.Active = false
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
	buffer := make([]byte, 1024)
	n, err := image.StdoutRead.Read(buffer)
	if err != nil {
		fmt.Println("initial stdoutread:", err)
		image.Active = false
		return
	}
	err = cn.WriteMessage(1, buffer[:n])
	fmt.Println("Got connect message: " + string(buffer[:n]))
	image.Waiting = false
	for {
		if !image.Active {
			break
		}
		n, err := image.StdoutRead.Read(buffer)
		if err != nil {
			fmt.Println("stdoutread:", err)
			break
		}
		fmt.Println(timeOfCreation, "GOT INPUT FROM CLIENT YO", string(buffer))
		err = cn.WriteMessage(1, buffer[:n])
		if err != nil {
			fmt.Println("write:", err)
			break
		}
		if string(buffer[:n]) == "exit" {
			break
		}
	}
	image.Active = false
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
		if !image.Active {
			break
		}
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
	image.Active = false
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
		if !image.Active {
			break
		}
		_, message, err := cn.ReadMessage()
		if err != nil {
			fmt.Println("read 3:", err)
			if !prevError {
				teamID := c.Param("id")
				imageName := c.Param("image")
				fmt.Println("REFRESHING DUE TO READ ERROR!")
				refreshShell(teamID, imageName, image)
				prevError = true
				continue
			}
			break
		}
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
	image.Active = false
}
