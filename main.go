package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

func init() {
	// set Logrus as soon as main package is initialized
	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: time.RFC822,
		FullTimestamp:   true,
		ForceColors:     true,
	})

	logrus.SetOutput(os.Stdout)
}

func main() {
	// define and parse input flags
	username := flag.String("user", "", "How do we call you?")
	chatroom := flag.String("room", "", "What topic are interested in?")
	discovery := flag.String("discovery", "", "How do you want to discover your peers?")
	loglevel := flag.String("log", "info", "How far down does a rabbit hole go?")
	flag.Parse()

	// set log levels
	switch *loglevel {
	case "info", "INFO":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn", "WARN":
		logrus.SetLevel(logrus.WarnLevel)
	case "error", "ERROR":
		logrus.SetLevel(logrus.ErrorLevel)
	case "trace", "TRACE":
		logrus.SetLevel(logrus.TraceLevel)
	case "debug", "DEBUG":
		logrus.SetLevel(logrus.DebugLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}

	// some welcoming display
	fmt.Println("P2Pchat is starting... Be with you shortly...")
	fmt.Println()

	// crete new P2P node host
	p2p := NewP2P()
	logrus.Infoln("Service Peers connected")

	// use chosen discovery method to connect peers
	switch *discovery {
	case "announce":
		p2p.AnnounceConnect()
	case "advertise":
		p2p.AdvertiseConnect()
	default:
		p2p.AnnounceConnect()
	}

	logrus.Infoln("Service Peers connected")

	// join chat room
	chatApp, _ := JoinChatRoom(p2p, *username, *chatroom)

	logrus.Infof("Joined the -> %s <- chatroom as -> %s", chatApp.RoomName, chatApp.Username)

	// wait for setup to complete
	time.Sleep(time.Second * 5)

	// render Chat UI
	ui := NewUI(chatApp)
	ui.Run()
}
