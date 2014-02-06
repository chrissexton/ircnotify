package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"regexp"

	"bitbucket.org/kisom/gopush/pushover"
	"github.com/kballard/goirc/irc"
)

var identity = pushover.Identity{}
var regexps []*regexp.Regexp
var paused = false
var playbackStart *regexp.Regexp
var playbackEnd *regexp.Regexp

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

	for _, val := range flag.Args() {
		if r, err := regexp.Compile(val); err != nil {
			log.Println("Couldn't parse regexp:", val)
		} else {
			regexps = append(regexps, r)
		}
	}

	playbackStart, _ = regexp.Compile("Buffer Playback")
	playbackEnd, _ = regexp.Compile("Playback Complete")

	quit := make(chan bool, 1)

	identity.Token = *token
	identity.User = *userKey

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
			hr.AddHandler(irc.CONNECTED, h_LoggedIn)
			hr.AddHandler(irc.DISCONNECTED, func(*irc.Conn, irc.Line) {
				log.Println("disconnected")
				quit <- true
			})
			hr.AddHandler("PRIVMSG", h_PRIVMSG)
			hr.AddHandler(irc.ACTION, h_ACTION)
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

func h_LoggedIn(conn *irc.Conn, line irc.Line) {
	log.Println("Finished connect")
}

func h_PRIVMSG(conn *irc.Conn, line irc.Line) {
	log.Printf("[%s] %s> %s\n", line.Args[0], line.Src, line.Args[1])

	checkMsg(line.Args[0], line.Src.Nick, line.Args[1])

	if line.Args[1] == "!quit" {
		conn.Quit("")
	}
}

func h_ACTION(conn *irc.Conn, line irc.Line) {
	log.Printf("[%s] %s %s\n", line.Dst, line.Src, line.Args[0])
	checkMsg(line.Dst, line.Src.Nick, line.Args[0])
}

func checkMsg(channel, sender, message string) {
	if playbackStart.MatchString(message) {
		paused = true
		log.Println("Pausing for buffer playback")
	} else if playbackEnd.MatchString(message) {
		paused = false
		log.Println("Unpausing from buffer playback")
	}

	if !paused {
		for _, r := range regexps {
			if r.MatchString(message) {
				msg := fmt.Sprintf("%s> %s", sender, message)
				pushover.Notify_titled(identity, msg, channel)
			}
		}
	}
}
