package util

import (
	"encoding/base64"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mohamed-rafraf/k8s-auth-kube/config"
)

func Connect() error {

	go func() {
		for {
			err := DeleteRowByTerminationTime()
			if err != nil {
				log.Println(err)
			}
			time.Sleep(1 * time.Second) // Wait for 1 minute before running the function again
		}
	}()

	// This creates a URL object with the address of the WebSocket server we want to connect to.
	u := url.URL{Scheme: "ws", Host: strings.Split(config.Config.Server, "//")[1], Path: "/ws"}
	log.Println(strings.Split(config.Config.Server, "//")[1])
	// The RawQuery parameter is used to specify a query string with parameters, in this case the cluster name.
	q := u.Query()
	q.Set("clusterName", config.Config.ClusterName)
	q.Set("token", config.Config.Token)
	u.RawQuery = q.Encode()

	// This establishes the WebSocket connection with the server using the URL object we just created.
	// The Dial function returns the WebSocket connection object "c", a response object and an error, which we are ignoring for now.
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)

	// If there is an error while establishing the connection, we log it and exit the program.
	if err != nil {
		return err

	}
	// We defer closing the connection until the end of the function to make sure it's properly closed.
	defer c.Close()

	log.Println("Connected to the authentication server!")

	for {
		messageType, message, err := c.ReadMessage()
		// If there is an error while reading the message
		if err != nil {
			return err

		}

		// If the message is binary, we assume it's a file and write it to disk.
		if messageType == websocket.BinaryMessage {

			err = os.WriteFile("file.txt", message, 0644)
			if err != nil {
				return err
			}
		} else { // If the message is not binary, we assume it's a text message

			if strings.Contains(string(message), "Token-Create") {
				token, err := GenerateToken(string(message))
				if err != nil {
					return err
				}
				cacrt := base64.StdEncoding.EncodeToString([]byte(config.Config.CaCrt))
				sendMessage(c, token+"\n---\n"+cacrt+"\n---\n"+config.Config.API)
			}

			if strings.Contains(string(message), "Token-Delete") {
				err := ClearToken(string(message))
				if err != nil {
					return err
				}
				sendMessage(c, "GOOD BYE!")
			}

		}
	}

}

func sendMessage(conn *websocket.Conn, message string) error {
	err := conn.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		return err
	}
	return nil
}
