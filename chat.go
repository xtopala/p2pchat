package main

import (
	"context"
	"encoding/json"
)

// default fallback user and chat room names
const defaultUsername = "anon"
const defaultChatRoom = "lobby"

type chatMessage struct {
	Text       string `json:"text"`
	SenderID   string `json:"senderId"`
	SenderName string `json:"senderName"`
}

type chatLog struct {
	logPrefix string
	logMsg    string
}

// this structure represents a PubSub Chat Room
type ChatRoom struct {
	// TODO: add Chat room p2p host

	// the channel for incomming messages
	Incomming chan chatMessage
	// the channel for outgoing messages
	Outgoing chan string
	// the channel for chat log messages
	Logs chan chatLog

	RoomName string
	UserName string

	// chat room lifecycle context
	ctx context.Context
	// chat room lifecycle cancellation function
	cancel context.CancelFunc
}

// This is a constuctor function which returns a new Chat Room
// for a given P2P host, username and room
func JoinChatRoom(p2p interface{}, username string, room string) (*ChatRoom, error) { return nil, nil }

// Method that publishes chat messages, and
// does so in a loop until the pubsub context is canceled
func (cr *ChatRoom) PubMessages() {
	for {
		select {
		case <-cr.ctx.Done():
			return
		case msgTxt := <-cr.Outgoing:
			// create a chat message
			chatMsg := chatMessage{
				Text:       msgTxt,
				SenderName: cr.UserName,
				// SenderID: ,
			}

			// serialize the chat message into JSON
			_, err := json.Marshal(chatMsg)
			if err != nil {
				cr.Logs <- chatLog{
					logPrefix: "err",
					logMsg:    "could not marshal JSON",
				}
				continue
			}

			// TODO: publish the message to the topic
		}
	}
}

// Method that contiously reads messages from the subscription
// and does so in a loop untill either the subscription or pubsub
// context is canceled.
// Received messages are parsed into the Incomming chanel
func (cr *ChatRoom) ReadSub() {
	for {
		select {
		case <-cr.ctx.Done():
			return
		default:
			// TODO: read messages we're subscribed to
		}
	}
}

// Method that returns a list of all peer IDs
// connected to the Chat Room
func (cr *ChatRoom) GetPeers() {}

// Method for unsubscribing from the topic
func (cr *ChatRoom) Leave() {}

// Method for updating the username
func (cr *ChatRoom) UpdateUser(username string) {}
