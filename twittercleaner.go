package main

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"flag"
	"log"
	"math"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/ChimeraCoder/anaconda"
)

type Configuration struct {
	Key         string
	Secret      string
	Token       string
	TokenSecret string
	MaxAge      int
}

func blindDeleter(config Configuration, c <-chan int64) {
	// Connect to Twitter
	anaconda.SetConsumerKey(config.Key)
	anaconda.SetConsumerSecret(config.Secret)
	api := anaconda.NewTwitterApi(config.Token, config.TokenSecret)
	for tweetid := range c {
		_, err := api.DeleteTweet(tweetid, true)
		if err != nil {
			log.Println(err)
		}
	}
}

func deleteOldTweetsFromTimeline(config Configuration, c chan int64) {
	// Connect to Twitter
	anaconda.SetConsumerKey(config.Key)
	anaconda.SetConsumerSecret(config.Secret)
	api := anaconda.NewTwitterApi(config.Token, config.TokenSecret)
	v := url.Values{}
	v.Set("count", "200")
	lasttweet := int64(math.MaxInt64)
	for {
		timeline, err := api.GetUserTimeline(v)
		if len(timeline) == 0 {
			break
		}
		if err != nil {
			log.Println(err)
			break
		}
		for _, tweet := range timeline {
			if tweet.Id < lasttweet {
				lasttweet = tweet.Id
			}

			v.Set("max_id", strconv.FormatInt(lasttweet-1, 10))
			ts, err := time.Parse("Mon Jan _2 15:04:05 -0700 2006", tweet.CreatedAt)
			if err != nil {
				log.Println(err)
			}
			if ts.Before(time.Now().Add(-time.Duration(config.MaxAge) * time.Hour)) {
				c <- tweet.Id
			}
		}
	}

}

func deleteOldTweetsFromArchive(config Configuration, arch string) {
	// Connect to Twitter
	anaconda.SetConsumerKey(config.Key)
	anaconda.SetConsumerSecret(config.Secret)
	api := anaconda.NewTwitterApi(config.Token, config.TokenSecret)
	r, err := zip.OpenReader(arch)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == "tweets.csv" {
			datafile, err := f.Open()
			if err != nil {
				log.Fatal(err)
			}
			reader := csv.NewReader(datafile)
			records, err := reader.ReadAll()
			if err != nil {
				log.Fatal(err)
			}
			for _, record := range records[1:] {
				id, err := strconv.ParseInt(record[0], 10, 64)
				if err != nil {
					log.Println(err)
				}
				_, err = api.DeleteTweet(id, true)
				if err != nil {
					log.Println(err)
				}
			}

		}
	}
}

func main() {

	// Parse commandline flags
	configLocation := flag.String("config", "/home/floort/.twittercleaner.json", "Location of the config file")
	consumerKey := flag.String("consumerkey", "", "Twitter Consumer Key")
	consumerSecret := flag.String("consumersecret", "", "Twitter Consumer Secret")
	twitterAccesToken := flag.String("accestoken", "", "Twitter Access Token")
	twitterAccesTokenSecret := flag.String("accesstokensecret", "", "Twitter Access Token Secret")
	twitterArchive := flag.String("archive", "", "Delete from twitter archive")
	maxAge := flag.Int("maxage", 48, "Maximum age of tweets (hours)")
	writeConfig := flag.Bool("writeconfig", false, "Write flags to configuration file")
	flag.Parse()

	config := Configuration{}
	if *writeConfig {
		// Write the configuration
		config.Key = *consumerKey
		config.Secret = *consumerSecret
		config.Token = *twitterAccesToken
		config.TokenSecret = *twitterAccesTokenSecret
		config.MaxAge = *maxAge

		file, err := os.Create(*configLocation)
		if err != nil {
			log.Fatal(err)
		}
		b, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		_, err = file.Write(b)
		if err != nil {
			log.Fatal(err)
		}
		file.Close()
	} else {
		// Read the configuration
		file, err := os.Open(*configLocation)
		if err != nil {
			log.Fatal(err)
		}
		decoder := json.NewDecoder(file)
		decoder.Decode(&config)
		file.Close()
		// Overwrite the config file with commandline flags
		if *consumerKey != "" {
			config.Key = *consumerKey
		}
		if *consumerSecret != "" {
			config.Secret = *consumerSecret
		}
		if *twitterAccesToken != "" {
			config.Token = *twitterAccesToken
		}
		if *twitterAccesTokenSecret != "" {
			config.TokenSecret = *twitterAccesTokenSecret
		}
		if *maxAge != 0 {
			config.MaxAge = *maxAge
		}

	}

	if *twitterArchive != "" {
		// Delete tweets from archive
		deleteOldTweetsFromArchive(config, *twitterArchive)
	} else {
		for c := time.Tick(1 * time.Hour); ; <-c { // Clean once an hour
			deletechan := make(chan int64)
			go blindDeleter(config, deletechan)
			go deleteOldTweetsFromTimeline(config, deletechan)
		}
	}
}
