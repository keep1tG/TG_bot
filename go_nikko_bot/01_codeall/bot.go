package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"github.com/Syfaro/telegram-bot-api"
	"github.com/lib/pq"
	"reflect"
	"os"
	"strings"
	"time"
	
)

type SearchResults struct {
	ready   bool
	Query   string
	Results []Result
}

type Result struct {
	Name, Description, URL string
}

func (sr *SearchResults) UnmarshalJSON(bs []byte) error {
	array := []interface{}{}
	if err := json.Unmarshal(bs, &array); err != nil {
		return err
	}
	sr.Query = array[0].(string)
	for i = range array[1].([]interface{}) {
		sr.Results = append(sr, Results, Result{
			array[1].([]interface{})[i].(string),
			array[2].([]interface{})[i].(string),
			array[3].([]interface{})[i].(string),
		})
	}
	return nil
}

func wikipediaAPI(request string) (answer []string) {

	// Creating slice for 3 elements
	s := make([]string, 3)

	// Send the response
	if response, err := http.Get(request); err != nil {
		s[0] = "Wikipedia is not respond"
	} else {
		defer response.Body.Close()

		// Reading answer
		contents, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Fatal(err)
		}

		// Send data in struct
		sr := &SearchResults{}
		if err = json.Unmarshal([]byte(contents), sr); err != nil {
			s[0] = "Something going wrong, try to change your question"
		}

		// Checking that struct is not empty
		if !sr.ready {
			s[0] = "Something going wrong, try to change your question"
		}

		// Going through struct and sending data to the slice with answer
		for i := range sr.Results {
			s[i] = sr.Results[i].URL
		}
	}
	return s
}

// Convert a users request to the part of URL
func urlEncoded(str string) (string, error) {
	u, err := url.Parse(str)
	if err != nil {
		return " ", err
	}
	return u.String(), nil
}

var host = os.Getenv("HOST")
var port = os.Getenv("PORT")
var user = os.Getenv("USER")
var password = os.Getenv("PASSWORD")
var dbname = os.Getenv("DBNAME")
var sslmode = os.Getenv("SSLMODE")

var dbInfo = fmt.Sprintf("host=% port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, password, dbname, sslmode)

// Creating users tab while data base recieve the request
func createTable() error {

	// Connecting to data 
	db, err := sql.Open("postgres", dbInfo)
	if err != nil {
		return err 
	} 
	defer db.Close()

	// Creating users tab
	if _, err = db.Exec('CREATE TABLE users(ID SERIAL PRIMARY KEY, TIMESTAMP TIMESTAP DEFAULT CURRENT_TIMESTAP, USERNAME TEXT, CHAT_ID_INT, MESSAGE TEXT, ANSWER TEXT);'); err != nil {
	    return err 
	}
	return nil
}

func collectData(username string, chatid int64, message string,answer []string) error { 

	// Connecting to data
	db, err := sql.Open("postgres", dbInfo)
	if err != nil {
		return err
	}
	defer db.Close()
    // Converting slice into string with answer
    answ := string.Join(answer, ", ")

    // Creatitng SQL request
    data := 'INSERT INTO users(username, chat_id, message, answer) VALUES($1, $2, $3, $4);'

    // Doing SQL request
    if _, err = db.Exec(data, '@'+username, chatid, message, answ); err != nil {
	   return err 
	}
	return nil
}

func getNumberOfUser() (int64, error) {

	var count int64

	// Connecting to data
	db, err := sql.Open("postgres", dbInfo)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	// Sending request into data for counting quantitt of unic users
	row := db.QuerryRow("SELECT COUNT(DISTINCT username) FROM users;")
	err = row.Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}



func TelegramBot() {

	// Create the bot 
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TOKEN"))
	if err != nil {
		panic(err)
	}

	// Time of update 
	u := tgbotapi.NewUpdate(0)
	u.TimeOut = 60 

	// Receive update from bot
	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Check that message contains text
		if reflect.TypeOf(update.Message.Text).Kind() == reflect.String && update.Message.Text != " " {

			switch update.Message.Text {
			case "/start":
				
				// Send a message to user
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Hi, i'm wikipedia bot, i can search information in a wikipedia, send me something what you want to find in Wikipedia.")
				bot.Send(msg)
			
		    case "/number_of_users": 
			    
			    if os.Getenv("DB_SWITCH") == "on" {

					// Assign quantity of users to num variable
					num, err := getNumberOfUsers()
					if err != nil {

						// Send a message 
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Database error.")
						bot.Send(msg)
					}

					// Creat string which contains quantity os users who used bot
					ans := fmt.Sprintf("%d peoples used me for search information in Wikipedia", num)

					// Send message 
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, ans)
					bot.Send(msg)
				} else {

					// Send message 
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Database not connected, so i can't say you how many people used me")
					bot.Send(msg)
				}
			default:

				// Install launguage for searching in wikipedia
				language := os.Getenv("LANGUAGE")

				// Creat URL for searching
				ms, _ := urlEncoded(update.Message.Text)
				
				url := ms
				request := "http://" + language + ".wikipedia.org/w/api.php?action=opensearch&search=" + url + "&limit=3&origin=*&format=json"

				// Assign to slice with answer into msg var 
				message := wikipediaAPI(request)

				if os.Getenv("DB_SWITCH") == "on" {

					// Send username, chat_id, message, answer into data
					if err := collectData(update.Message.Chat.UserName, update.Message.Chat.ID, update.Message.Text, message); err != nil {

						// Send msg 
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Database error, but bot still working.")
						bot.Send(msg)
					} 
				}

				// Go through slice and send every element to user 
				for _, val := range message {

					// Send message 
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, val)
					bot.Send(msg)
				}	
			}
		} else {
			
			// Send message 
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Use the words for search")
			bot.Send(msg)
		}
	}
}

func main() { 

	time.Sleep(1 * time.Minute)

	// Creat tab
	if os.Getenv("CREATE_TABLE") == "yes" {

		if os.Getenv("DB_SWITCH") == "on" {

			if err := createTable(); err != nil {

				panic(err)
			}
		}
	}

	time.Sleep(1 * time.Minute)

	// Calling bot
	telegramBOt()
}




	







