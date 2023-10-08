package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

// Sent reqest to twitch api, searching for streamer with passed name.
// For each request we create seperate client.
func sent(streamer_name string) (map[string]string, error) {
	url := "https://api.twitch.tv/helix/search/channels?query=" + streamer_name
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Client-Id", os.Getenv("CLIENT_ID"))
	req.Header.Add("Authorization", "Bearer "+os.Getenv("BEARER_TOKEN"))
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var b map[string][]map[string]string
	json.Unmarshal(body, &b)
	var result map[string]string
	// Go thourgh the response to find the exact streamer.
	for index, user := range b["data"] {
		if strings.Compare(user["broadcaster_login"], streamer_name) == 0 {
			result = b["data"][index]
		}
	}
	return result, nil

}

// Create a goroutine that for each name in the array of names will output the result into
// out channel
func group_streamer(name string) <-chan map[string]string {
	out := make(chan map[string]string, 1)
	go func() {
		data, _ := sent(name)
		out <- data
		close(out)
	}()
	return out
}

// Merge all out channels into one (Fan-In function)
func merge(done chan struct{}, cs ...<-chan map[string]string) <-chan map[string]string {
	var wg sync.WaitGroup
	out := make(chan map[string]string, 1)

	output := func(c <-chan map[string]string) {
		defer wg.Done()

		for n := range c {
			select {
			case out <- n:
			case <-done:
				return
			}
		}
	}

	wg.Add(len(cs))
	for _, c := range cs {
		go output(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func main() {
	godotenv.Load(".env")
	done := make(chan struct{}, 2)
	var chans []<-chan map[string]string

	err := get_auth()
	if err != nil {
		fmt.Println("AUTH", err)
		return
	}

	names := []string{"tekken", "vinesauce", "scottyfionnghall"}
	for _, name := range names {
		chans = append(chans, group_streamer(name))
	}

	for n := range merge(done, chans...) {
		fmt.Println(n)
	}
}

// Get authentication berear token and add it to the environment variables.
func get_auth() error {
	url := "https://id.twitch.tv/oauth2/token?client_id=" + os.Getenv("CLIENT_ID") + "&client_secret=" + os.Getenv("CLIENT_SECRET") + "&grant_type=client_credentials"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var b map[string]string
	json.Unmarshal(body, &b)
	os.Setenv("BEARER_TOKEN", b["access_token"])
	return nil
}
