package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	Token         = "YOUR_BOT_TOKEN"
	WeatherApiKey = "OPEN_WEATHER_API"
	CommandMap    = make(map[string]func(s *discordgo.Session, m *discordgo.MessageCreate, args []string))
	Reminders     = make(map[string]time.Time)
)

func main() {
	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	dg.AddHandler(messageCreate)

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	fmt.Println("Bot is now running. Press CTRL+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	fmt.Println("Message received:", m.Content) // Debugging line

	if strings.HasPrefix(m.Content, "!help") {
		helpMessage := "Available Commands:\n" +
			"!poll - Create a new poll. Format: !poll Question? Option1; Option2; Option3\n" +
			"!weather - Find out weather condition. Format: !weather cityname\n" +
			"!reminder - Set a reminder. Format: !reminder <time_in_minutes> <message>"
		s.ChannelMessageSend(m.ChannelID, helpMessage)
		return
	}

	if strings.HasPrefix(m.Content, "!poll") {
		createPoll(s, m)
		return
	}
	if strings.HasPrefix(m.Content, "!weather") {
		getWeather(s, m)
		return
	}
	if strings.HasPrefix(m.Content, "!reminder") {
		commandReminder(s, m)
		return
	}
}

func getWeather(s *discordgo.Session, m *discordgo.MessageCreate) {
	city := strings.TrimSpace(strings.TrimPrefix(m.Content, "!weather"))
	if city == "" {
		s.ChannelMessageSend(m.ChannelID, "Please specify a city.")
		return
	}

	url := fmt.Sprintf("http://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=metric", city, WeatherApiKey)
	resp, err := http.Get(url)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Error fetching weather data.")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		s.ChannelMessageSend(m.ChannelID, "Error fetching weather data.")
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Error reading weather data.")
		return
	}

	var data map[string]interface{}
	json.Unmarshal(body, &data)

	if data["weather"] == nil {
		s.ChannelMessageSend(m.ChannelID, "Could not find weather data for that city.")
		return
	}

	weather := data["weather"].([]interface{})[0].(map[string]interface{})
	main := data["main"].(map[string]interface{})
	message := fmt.Sprintf("Weather in %s: %s, %v°C", city, weather["description"], main["temp"])

	s.ChannelMessageSend(m.ChannelID, message)
}

func createPoll(s *discordgo.Session, m *discordgo.MessageCreate) {
	content := strings.TrimPrefix(m.Content, "!poll ")
	parts := strings.Split(content, "?")
	if len(parts) != 2 {
		s.ChannelMessageSend(m.ChannelID, "Invalid poll format. Use: !poll Question? Option1; Option2; Option3")
		return
	}

	question := parts[0]
	options := strings.Split(parts[1], ";")
	if len(options) < 2 {
		s.ChannelMessageSend(m.ChannelID, "A poll must have at least two options.")
		return
	}

	pollMessage := "Poll created:\n**" + question + "?**\n"
	for i, option := range options {
		pollMessage += fmt.Sprintf("%d. %s\n", i+1, strings.TrimSpace(option))
	}

	s.ChannelMessageSend(m.ChannelID, pollMessage)
}

func commandReminder(s *discordgo.Session, m *discordgo.MessageCreate) {
	content := strings.TrimPrefix(m.Content, "!reminder ")
	parts := strings.SplitN(content, " ", 2)
	if len(parts) != 2 {
		s.ChannelMessageSend(m.ChannelID, "Usage: !reminder <time_in_minutes> <message>")
		return
	}

	minutesStr := parts[0]
	message := parts[1]

	minutes, err := strconv.Atoi(minutesStr)
	if err != nil || minutes <= 0 {
		s.ChannelMessageSend(m.ChannelID, "Invalid time format. Please use a positive integer for minutes.")
		return
	}

	reminderTime := time.Now().Add(time.Duration(minutes) * time.Minute)

	Reminders[m.Author.ID] = reminderTime

	_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Reminder set for %s: %s", reminderTime.Format("15:04:05"), message))
	if err != nil {
		fmt.Println("Error sending reminder message:", err)
	}

	go func() {
		time.Sleep(time.Until(reminderTime))
		delete(Reminders, m.Author.ID)
		_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s>, Reminder: %s", m.Author.ID, message))
		if err != nil {
			fmt.Println("Error sending reminder:", err)
		}
	}()
}
