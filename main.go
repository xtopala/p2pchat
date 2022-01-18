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
	case "trace", "TRACE":
		logrus.SetLevel(logrus.TraceLevel)
	case "warn", "WARN":
		logrus.SetLevel(logrus.WarnLevel)
	case "error", "ERROR":
		logrus.SetLevel(logrus.ErrorLevel)
	}

	// some welcoming display
	fmt.Println("P2Pchat is starting... Be with you shortly...")
	fmt.Println()

	// use chosen discovery method to connect peers
	switch *discovery {
	default:
		//
	}

	// join chat room
	fmt.Println(username)
	fmt.Println(chatroom)

	// render Chat UI
}
