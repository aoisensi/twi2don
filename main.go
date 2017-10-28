package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/mattn/go-mastodon"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"

	"gopkg.in/yaml.v2"
)

var config Config

func main() {
	configData, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Fatalln(err)
	}
	if err := yaml.Unmarshal(configData, &config); err != nil {
		log.Fatalln(err)
	}

	twiConfig := oauth1.NewConfig(config.Twitter.ConsumerKey, config.Twitter.ConsumerSecret)
	twiToken := oauth1.NewToken(config.Twitter.AccessKey, config.Twitter.AccessSecret)
	httpClient := twiConfig.Client(oauth1.NoContext, twiToken)

	twitterClient := twitter.NewClient(httpClient)

	for i, relay := range config.Relays {
		client := mastodon.NewClient(&mastodon.Config{
			Server:      relay.Mastodon.Server,
			AccessToken: relay.Mastodon.AccessToken,
		})
		config.Relays[i].MastodonClient = client
	}

	demux := twitter.NewSwitchDemux()
	demux.Tweet = func(tweet *twitter.Tweet) {
		for _, relay := range config.Relays {
			if relay.Twitter.ScreenName != tweet.User.ScreenName {
				continue
			}
			if tweet.Retweeted {
				break
			}
			status := &mastodon.Toot{
				Status: fmt.Sprintf(
					"%v\n\nhttps://twitter.com/%v/status/%v",
					tweet.Text, tweet.User.ScreenName, tweet.IDStr,
				),
			}
			toot, err := relay.MastodonClient.PostStatus(context.Background(), status)
			if err != nil {
				log.Println("Failed toot.")
				log.Println(err)
				break
			}
			log.Println("Tooted!")
			log.Println(toot.URL)
			break
		}
	}

	if _, _, err := twitterClient.Accounts.VerifyCredentials(nil); err != nil {
		log.Fatalln(err)
	}

	stream, err := twitterClient.Streams.User(&twitter.StreamUserParams{
		With: "followings",
	})
	if err != nil {
		log.Fatalln(err)
	}
	for message := range stream.Messages {
		demux.Handle(message)
	}
}

type Config struct {
	Twitter struct {
		ConsumerKey    string `yaml:"consumer_key"`
		ConsumerSecret string `yaml:"consumer_secret"`
		AccessKey      string `yaml:"access_key"`
		AccessSecret   string `yaml:"access_secret"`
	} `yaml:"twitter"`
	Relays []struct {
		Mastodon struct {
			Server      string `yaml:"server"`
			AccessToken string `yaml:"access_token"`
		} `yaml:"mastodon"`
		Twitter struct {
			ScreenName string `yaml:"screen_name"`
		} `yaml:"twitter"`
		MastodonClient *mastodon.Client `yaml:"-"`
	} `yaml:"relays"`
}
