package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// default fallback user and chat room names
const defaultUsername = "anon"
const defaultRoomName = "lobby"

type chatMessage struct {
	Message    string `json:"message"`
	SenderID   string `json:"senderId"`
	SenderName string `json:"senderName"`
}

type chatLog struct {
	logPrefix string
	logMsg    string
}

// this structure represents a PubSub Chat Room
type ChatRoom struct {
	// P2P host for the Chat Room
	Host *P2P

	// the channel for incomming messages
	Incomming chan chatMessage
	// the channel for outgoing messages
	Outgoing chan string
	// the channel for chat log messages
	Logs chan chatLog

	RoomName string
	Username string
	// host ID of the Peer
	selfID peer.ID

	// chat room lifecycle context
	ctx context.Context
	// chat room lifecycle cancellation function
	cancel context.CancelFunc
	// PubSub topic of the Chat Room
	topic *pubsub.Topic
	// PubSub subscription for the topic
	subscription *pubsub.Subscription
}

// This is a constuctor function which returns a new Chat Room
// for a given P2P host, username and room
func JoinChatRoom(p2p *P2P, username string, roomName string) (*ChatRoom, error) {
	// create PubSub topic with the room name
	topic, err := p2p.PubSub.Join(fmt.Sprintf("p2p-room-%s", roomName))
	if err != nil {
		return nil, err
	}

	// subscribe to the PubSub topic
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	if len(username) == 0 {
		username = defaultUsername
	}

	if len(roomName) == 0 {
		roomName = defaultRoomName
	}

	// create cancellable context
	pubSubCtx, cancel := context.WithCancel(context.Background())

	chatRoom := &ChatRoom{
		Host: p2p,

		Incomming: make(chan chatMessage),
		Outgoing:  make(chan string),
		Logs:      make(chan chatLog),

		ctx:          pubSubCtx,
		cancel:       cancel,
		topic:        topic,
		subscription: sub,

		RoomName: roomName,
		Username: username,
		selfID:   p2p.Host.ID(),
	}

	// start reading subscribtions
	go chatRoom.ReadSub()
	// start publishing
	go chatRoom.PubMessages()

	return chatRoom, nil
}

// Method that publishes chat messages, and
// does so in a loop until the pubsub context is canceled
func (cr *ChatRoom) PubMessages() {
	for {
		select {
		case <-cr.ctx.Done():
			return

		case msg := <-cr.Outgoing:
			// create a chat message
			chatMsg := chatMessage{
				Message:    msg,
				SenderName: cr.Username,
				SenderID:   cr.selfID.Pretty(),
			}

			// serialize the chat message into JSON
			msgBytes, err := json.Marshal(chatMsg)
			if err != nil {
				cr.Logs <- chatLog{
					logPrefix: "puberr",
					logMsg:    "could not marshal JSON",
				}
				continue
			}

			if err := cr.topic.Publish(cr.ctx, msgBytes); err != nil {
				cr.Logs <- chatLog{
					logPrefix: "puberr",
					logMsg:    "could not publish message to topic",
				}
				continue
			}
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
			// read a message from the subscription
			msg, err := cr.subscription.Next(cr.ctx)
			if err != nil {
				// close the messages queue (subscription has closed)
				close(cr.Incomming)
				cr.Logs <- chatLog{
					logPrefix: "suberr",
					logMsg:    "subscription has closed",
				}
				return
			}

			// check if message is from self
			if msg.ReceivedFrom == cr.selfID {
				continue
			}

			cm := &chatMessage{}
			err = json.Unmarshal(msg.Data, cm)
			if err != nil {
				cr.Logs <- chatLog{
					logPrefix: "suberr",
					logMsg:    "could not unmarshal JSON",
				}
				continue
			}

			// send the Chat message into the message queue
			cr.Incomming <- *cm
		}
	}
}

// Method that returns a list of all peer IDs
// connected to the Chat Room
func (cr *ChatRoom) GetPeers() []peer.ID {
	return cr.topic.ListPeers()
}

// Method for unsubscribing from the topic
func (cr *ChatRoom) Leave() {
	defer cr.cancel()

	// cancel the existing subscription
	cr.subscription.Cancel()
	// close the topic handler
	cr.topic.Close()
}

// Method for updating the username
func (cr *ChatRoom) UpdateUser(username string) {
	cr.Username = username
}
