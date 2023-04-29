package util

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/mohamed-rafraf/k8s-auth-kube/config"
)

func Verify() error {
	link := config.Config.Server + "/verify?token=" + config.Config.Token + "&cluster=" + config.Config.ClusterName + "&api=" + config.Config.API
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	// Create a new request with POST method
	req, err := http.NewRequest("POST", link, nil)
	if err != nil {
		return errors.New("can't Send Request to Server " + config.Config.Server)

	}

	// Add the authentication header and bearer token to the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println(err)
		return errors.New("can't reach the authentication server" + config.Config.Server)
	}

	defer resp.Body.Close()

	// Read the response body
	var response Response
	dat, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(dat, &response)
	if err != nil {
		return errors.New("bad data format")
	}

	if response.Status == "fail" {
		return errors.New(response.Message)
	}

	return nil
}
