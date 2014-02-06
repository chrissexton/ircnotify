// © 2014 the ircnotify Authors under the WTFPL. See AUTHORS for the list of authors.

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"regexp"
	"strings"

	"bitbucket.org/kisom/gopush/pushover"
	"github.com/kballard/goirc/irc"
)

type notifyConfig struct {
	identity      pushover.Identity
	regexps       []*regexp.Regexp
	paused        bool
	playbackStart *regexp.Regexp
	playbackEnd   *regexp.Regexp
	nick          string
}

func main() {

	token := flag.String("token", "", "Pushover.net API token")
	userKey := flag.String("userKey", "", "Pushover.net user key")
	host := flag.String("host", "127.0.0.1", "IRC server to connect")
	port := flag.Uint("port", 6667, "IRC server port")
	ssl := flag.Bool("ssl", false, "Use SSL for IRC")
	password := flag.String("password", "", "Password for IRC server")
	nick := flag.String("nick", "ircuser", "Nick for IRC server")
	user := flag.String("user", "ircuser", "User for IRC server")
	realName := flag.String("realName", "ircuser", "Real name for IRC server")
	flag.Parse()

	quit := make(chan bool, 1)

	// Used in callbacks
	myConfig := &notifyConfig{
		identity: pushover.Identity{
			Token: *token,
			User:  *userKey,
		},
		regexps: make([]*regexp.Regexp, 0),
		paused:  false,
		nick:    *nick,
	}
	myConfig.playbackStart, _ = regexp.Compile("Buffer Playback")
	myConfig.playbackEnd, _ = regexp.Compile("Playback Complete")

	for _, val := range flag.Args() {
		myConfig.regexps = addRegexp(myConfig.regexps, val)
	}

	privmsg := func(conn *irc.Conn, line irc.Line) {
		message := line.Args[1]
		add, remove := "!add ", "!remove "
		if line.Src.Nick == *nick && strings.HasPrefix(message, add) {
			message = strings.Replace(message, add, "", 1)
			myConfig.regexps = addRegexp(myConfig.regexps, message)
			log.Println("Added regexp:", message)
		} else if line.Src.Nick == *nick && strings.HasPrefix(message, remove) {
			message = strings.Replace(message, remove, "", 1)
			for i, r := range myConfig.regexps {
				if r.String() == message {
					myConfig.regexps = append(myConfig.regexps[:i], myConfig.regexps[i+1:]...)
					log.Println("Removed regexp:", message)
				}
			}
		} else {
			checkMsg(myConfig, line.Args[0], line.Src.Nick, message)
		}
	}

	action := func(conn *irc.Conn, line irc.Line) {
		checkMsg(myConfig, line.Dst, line.Src.Nick, line.Args[0])
	}

	config := irc.Config{
		Host: *host,
		Port: *port,

		SSL: *ssl,
		SSLConfig: &tls.Config{
			InsecureSkipVerify: true,
		},

		Password: *password,
		Nick:     *nick,
		User:     *user,
		RealName: *realName,

		Init: func(hr irc.HandlerRegistry) {
			log.Println("init")
			hr.AddHandler(irc.CONNECTED, loggedIn)
			hr.AddHandler(irc.DISCONNECTED, func(*irc.Conn, irc.Line) {
				log.Println("disconnected")
				quit <- true
			})
			hr.AddHandler("PRIVMSG", privmsg)
			hr.AddHandler(irc.ACTION, action)
		},
	}

	log.Println("Connecting")
	if _, err := irc.Connect(config); err != nil {
		log.Println("error:", err)
		quit <- true
	}

	<-quit
	log.Println("Goodbye")
}

func loggedIn(conn *irc.Conn, line irc.Line) {
	log.Println("Finished connect")
}

func addRegexp(regexps []*regexp.Regexp, val string) []*regexp.Regexp {
	if r, err := regexp.Compile(val); err != nil {
		log.Println("Couldn't parse regexp:", val)
	} else {
		regexps = append(regexps, r)
	}
	return regexps
}

func checkMsg(config *notifyConfig, channel, sender, message string) {
	if config.playbackStart.MatchString(message) {
		config.paused = true
		log.Println("Pausing for buffer playback")
	} else if config.playbackEnd.MatchString(message) {
		config.paused = false
		log.Println("Unpausing from buffer playback")
	}

	if !config.paused {
		for _, r := range config.regexps {
			if r.MatchString(message) {
				msg := fmt.Sprintf("%s> %s", sender, message)
				pushover.Notify_titled(config.identity, msg, channel)
				log.Printf("Sending message: %s: %s", msg, channel)
			}
		}
	}
}
