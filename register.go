// Register slash commands

package main

import (
	"bytes"
	"fmt"
 	"log"
 	"os"
	"io/ioutil"
	"encoding/json"
	"net/http"

 	"github.com/joho/godotenv"
)

type Option struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        int    `json:"type"`
	Required    bool   `json:"required"`
}

type Command struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Options     []Option `json:"options"`
}

func main(){
 	err := godotenv.Load(".env")
 	if err != nil {
  		log.Fatalf("Error loading .env file: %s", err)
 	}
 	TOKEN := os.Getenv("BOT_TOKEN")

	file, err := os.Open("commands.json")
	if err != nil {
		log.Fatal("Error loading file:", err)
	}
	defer file.Close()

	fileContent, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatal("Error reading file:", err)
	}

	var commands []Command
	err = json.Unmarshal(fileContent, &commands)
	if err != nil {
		log.Fatal("Error parsing json file:", err)
	}

	url := "https://discord.com/api/v10/applications/1305095284774404116/commands"

	for _, command := range commands {
		commandJSON, err := json.Marshal(command)
		if err != nil {
			log.Fatal("Error marshalling command to JSON:", err)
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(commandJSON))
		if err != nil {
			log.Fatal("Error creating request for command", command.Name, "error:", err)
		}
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Authorization", "Bot " + TOKEN)

		client := &http.Client{}
		res, err := client.Do(req)
		if err != nil {
			log.Fatal("Error sending request for command", command.Name, "error:", err)
		}
		defer res.Body.Close()

		fmt.Println("Successfully uploaded command", command.Name, "with status code:", res.StatusCode)
	}
}