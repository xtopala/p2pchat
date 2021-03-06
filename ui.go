package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// UI represents what user sees in a Chat Room
type UI struct {
	*ChatRoom

	// tview application
	TerminalApp *tview.Application

	// user message input queue
	MsgInputs chan string
	// user command input queue
	CmdInputs chan uiCommand

	// UI element that lists peers
	peerList *tview.TextView
	// UI element with chat messages and logs
	messageList *tview.TextView
	// UI element for user input
	inputField *tview.InputField
}

// representation of a UI command
type uiCommand struct {
	cmdtype string
	cmdarg  string
}

// Constructor function for a new UI
func NewUI(cr *ChatRoom) *UI {
	// we need a new Tview app
	tapp := tview.NewApplication()

	// we need our message anc commands channels
	cmdchan := make(chan uiCommand)
	msgchan := make(chan string)

	// a nice title for our chat application
	titlebox := tview.NewTextView().
		SetText("PtwoP Chat").
		SetTextColor(tcell.ColorHotPink).
		SetTextAlign(tview.AlignCenter)
	// these can't be done in the same chain call,
	// since border setters return a different type, a Box type pointer, duuuh
	titlebox.
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen)

	// message list in a box to display messages and logs
	messageList := tview.NewTextView().
		SetDynamicColors(true).
		SetChangedFunc(func() { tapp.Draw() })

	messageList.
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitle(fmt.Sprintf("ChatRoom: %s", cr.RoomName)).
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(tcell.ColorPapayaWhip)

	// usage intructions
	usage := tview.NewTextView().
		SetDynamicColors(true).
		SetText(`[red]/quit[green] - quit the chat | [red]/room <roomname>[green] - change chat room | [red]/user <username>[green] - change user name | [red]/clear[green] - clear the chat`)

	usage.
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitle("Usage").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(tcell.ColorAntiqueWhite).
		SetBorderPadding(0, 0, 1, 0)

	// peer list displayed in a box
	peerList := tview.NewTextView()
	peerList.
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitle("Peers").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(tcell.ColorWhite)

	// text input box
	inputField := tview.NewInputField().
		SetLabel(fmt.Sprintf("%s > ", cr.Username)).
		SetLabelColor(tcell.ColorGreen).
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorBlack)

	inputField.
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitle("Input").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(tcell.ColorWhite).
		SetBorderPadding(0, 0, 1, 0)

	// define here what should happen when the input is done
	inputField.SetDoneFunc(func(key tcell.Key) {
		// check if trigger was caused by a Return(Enter) press
		if key != tcell.KeyEnter {
			return
		}

		// read the input text
		line := inputField.GetText()
		// no point printing empty messages
		if len(line) == 0 {
			return
		}

		// check for command inputs
		if strings.HasPrefix(line, "/") {
			cmdparts := strings.Split(line, " ")
			if len(cmdparts) == 1 {
				cmdparts = append(cmdparts, "")
			}

			// send the command
			cmdchan <- uiCommand{cmdtype: cmdparts[0], cmdarg: cmdparts[1]}

		} else {
			// send the message
			msgchan <- line
		}

		// reset the input field
		inputField.SetText("")
	})

	// flex container for message and peer boxes
	msgAndPeers := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(messageList, 0, 1, false).
		AddItem(peerList, 20, 1, false)

	// flexbox to fit all inside
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(titlebox, 3, 1, false).
		AddItem(msgAndPeers, 0, 8, false).
		AddItem(inputField, 3, 1, true).
		AddItem(usage, 3, 1, false)

	// set the flex as the app root
	tapp.SetRoot(flex, true)

	// return newly created UI
	return &UI{
		ChatRoom:    cr,
		TerminalApp: tapp,
		peerList:    peerList,
		messageList: messageList,
		inputField:  inputField,
		MsgInputs:   msgchan,
		CmdInputs:   cmdchan,
	}
}

// Method that starts the UI app
func (ui *UI) Run() error {
	go ui.eventHandler()
	defer ui.Close()

	return ui.TerminalApp.Run()
}

// Method that you know what it does
func (ui *UI) Close() {
	ui.cancel()
}

// Method that prints messages received from self
func (ui *UI) printSelfMessage(msg string) {
	prompt := fmt.Sprintf("[blue]<%s>:[-]", ui.Username)
	fmt.Fprintf(ui.messageList, "%s %s\n", prompt, msg)
}

// Method that prints messages received from a peer
func (ui *UI) printChatMessage(msg chatMessage) {
	prompt := fmt.Sprintf("[green]<%s>:[-]", msg.SenderName)
	fmt.Fprintf(ui.messageList, "%s %s\n", prompt, msg.Message)
}

// Method that prints log messages
func (ui *UI) printLogMessage(log chatLog) {
	prompt := fmt.Sprintf("[yellow]<%s>:[-]", log.logPrefix)
	fmt.Fprintf(ui.messageList, "%s %s\n", prompt, log.logMsg)
}

// Method that refreshes the listo of peers
func (ui *UI) syncPeerList() {
	// get all chatroom peers
	peers := ui.GetPeers()

	// acquire the thread lock
	ui.peerList.Lock()
	// clear the list
	ui.peerList.Clear()
	// release the lock
	ui.peerList.Unlock()

	for _, p := range peers {
		peerID := p.Pretty()
		// peerID is too long for display, nasty
		peerID = peerID[len(peerID)-8:]
		// add that pretty ID to the list
		fmt.Fprintln(ui.peerList, peerID)
	}

	// refresh the UI
	ui.TerminalApp.Draw()
}

func (ui *UI) handleCommand(cmd uiCommand) {
	switch cmd.cmdtype {
	case "/quit":
		// stop chatting, go home
		ui.TerminalApp.Stop()
		return

	case "/clear":
		// clear UI message box
		ui.messageList.Clear()

	case "/room":
		if len(cmd.cmdarg) == 0 {
			ui.Logs <- chatLog{logPrefix: "badcmd", logMsg: "missing room name for command"}
		} else {
			ui.Logs <- chatLog{logPrefix: "roomchange", logMsg: fmt.Sprintf("joining new room: %s", cmd.cmdarg)}

			oldChatRoom := ui.ChatRoom
			newChatRoom, err := JoinChatRoom(ui.Host, ui.Username, cmd.cmdarg)
			if err != nil {
				ui.Logs <- chatLog{logPrefix: "jumperr", logMsg: fmt.Sprintf("could not change room: %s", err)}
				return
			}

			ui.ChatRoom = newChatRoom
			// give time for queues to adapt
			time.Sleep(time.Second)

			oldChatRoom.Leave()

			ui.messageList.Clear()
			ui.messageList.SetTitle(fmt.Sprintf("ChatRoom: %s", ui.ChatRoom.RoomName))
		}

	case "/user":
		if len(cmd.cmdarg) == 0 {
			ui.Logs <- chatLog{logPrefix: "badcmd", logMsg: "missing user name for command"}
		} else {
			ui.UpdateUser(cmd.cmdarg)
			ui.inputField.SetLabel(fmt.Sprintf("%s > ", ui.Username))
		}

	default:
		ui.Logs <- chatLog{logPrefix: "badcmd", logMsg: fmt.Sprintf("unsupported command - %s", cmd.cmdtype)}
	}
}

// this will handle UI events
func (ui *UI) eventHandler() {
	refresh := time.NewTicker(time.Second)
	defer refresh.Stop()

	for {
		select {
		case msg := <-ui.MsgInputs:
			// send the message to outbound queue
			ui.Outgoing <- msg
			// add message to the message box as a message from myself
			ui.printSelfMessage(msg)

		case cmd := <-ui.CmdInputs:
			go ui.handleCommand(cmd)

		case msg := <-ui.Incomming:
			// print received messages to the message box
			ui.printChatMessage(msg)

		case log := <-ui.Logs:
			// display logs
			ui.printLogMessage(log)

		case <-refresh.C:
			// periodically refresh the peer list
			ui.syncPeerList()

		case <-ui.ctx.Done():
			// end event loop
			return
		}
	}
}
