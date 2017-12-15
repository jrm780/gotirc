// Package gotirc contains functions for connecting to Twitch.tv chat via IRC
package gotirc

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const sendBufferSize = 512

var caps = []string{"membership", "commands", "tags"}

// Options facilitates passing desired settings to a new Client
type Options struct {
	Debug     bool
	Port      int
	Host      string
	Channels  []string
	Reconnect bool
}

// Client holds state and context information to maintain a connection with a server
type Client struct {
	options Options

	sendQueue   chan string
	recvChannel chan Message
	reader      *bufio.Reader
	writer      *bufio.Writer

	conn        net.Conn
	readTimeout time.Duration

	callbackMu            sync.Mutex
	actionCallbacks       []func(channel string, tags map[string]string, msg string)
	chatCallbacks         []func(channel string, tags map[string]string, msg string)
	resubCallbacks        []func(channel string, tags map[string]string, msg string)
	subscriptionCallbacks []func(channel string, tags map[string]string, msg string)
	cheerCallbacks        []func(channel string, tags map[string]string, msg string)
	joinCallbacks         []func(channel, username string)
}

// NewClient returns a new Client
func NewClient(o Options) *Client {
	return &Client{
		options:     o,
		readTimeout: 10 * time.Minute,
	}
}

// Connect connects the client to the server specified in the options.
// This call will block and run event callbacks until disconnected
func (c *Client) Connect(nick string, pass string) error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", c.options.Host, c.options.Port))
	if err != nil {
		return err
	}

	err = c.doPostConnect(nick, pass, conn, 19, 30)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) doPostConnect(nick, pass string, conn net.Conn, maxMessages, perSeconds float64) error {
	c.reader = bufio.NewReader(conn)
	c.writer = bufio.NewWriter(conn)
	c.sendQueue = make(chan string, sendBufferSize)

	if err := c.authenticate(nick, pass); err != nil {
		return err
	}

	for _, channel := range c.options.Channels {
		c.Join(channel)
	}

	go c.startSendLoop(maxMessages, perSeconds)

	return c.startRecvLoop()
}

// OnAction adds an event callback for action (e.g., /me) messages
func (c *Client) OnAction(callback func(channel string, tags map[string]string, msg string)) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.actionCallbacks = append(c.actionCallbacks, callback)
}

// OnChat adds an event callback for when a user sends a message in a channel
func (c *Client) OnChat(callback func(channel string, tags map[string]string, msg string)) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.chatCallbacks = append(c.chatCallbacks, callback)
}

// OnResub adds an event callback for when a user resubs to a channel
func (c *Client) OnResub(callback func(channel string, tags map[string]string, msg string)) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.resubCallbacks = append(c.resubCallbacks, callback)
}

// OnSubscription adds an event callback for when a user subscribes to a channel
func (c *Client) OnSubscription(callback func(channel string, tags map[string]string, msg string)) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.subscriptionCallbacks = append(c.subscriptionCallbacks, callback)
}

// OnCheer adds an event callback for when a user cheers bits in a channel
func (c *Client) OnCheer(callback func(channel string, tags map[string]string, msg string)) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.cheerCallbacks = append(c.cheerCallbacks, callback)
}

// OnJoin adds an event callback for when a user joins a channel
func (c *Client) OnJoin(callback func(channel, username string)) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.joinCallbacks = append(c.joinCallbacks, callback)
}

// Join tells the client to join a particular channel. If the "#" prefix is missing,
// it is automatically prepended.
func (c *Client) Join(channel string) {
	if !strings.HasPrefix(channel, "#") {
		channel = "#" + channel
	}
	c.send("JOIN %s", channel)
}

func (c *Client) authenticate(nick, pass string) error {
	if err := c.write(fmt.Sprintf("PASS %s\r\n", pass)); err != nil {
		return err
	}
	if err := c.write(fmt.Sprintf("NICK %s\r\n", nick)); err != nil {
		return err
	}

	line, err := c.read()
	if err != nil {
		return err
	}

	msg := NewMessage(line)
	if msg.Command != "001" {
		return fmt.Errorf("Unexpected server response: %s", line)
	}

	c.write(fmt.Sprintf("CAP REQ :%s\r\n", strings.Join(caps, " twitch.tv/")))

	return nil
}

func (c *Client) send(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	select {
	case c.sendQueue <- msg:
	default:
		c.log("Send queue full; discarding message: %s", msg)
	}
}

func (c *Client) write(data string) error {
	c.log("< %s", data)
	c.conn.SetWriteDeadline(time.Now().Add(1 * time.Minute))
	_, err := c.writer.WriteString(data)
	if err != nil {
		return err
	}
	return c.writer.Flush()
}

func (c *Client) read() (string, error) {
	line, err := c.reader.ReadString('\n')
	c.log("> %s", line)
	return line, err
}

func (c *Client) log(format string, v ...interface{}) {
	if c.options.Debug {
		log.Printf(format, v...)
	}
}

func (c *Client) startSendLoop(maxMessages, perSeconds float64) {
	defer c.conn.Close()
	tokens := maxMessages
	lastTick := time.Now()

	for data := range c.sendQueue {

		if !strings.HasSuffix(data, "\r\n") {
			data = data + "\r\n"
		}

		now := time.Now()
		elapsedTime := now.Sub(lastTick)
		lastTick = now
		tokens += elapsedTime.Seconds() * (maxMessages / perSeconds)

		if tokens >= maxMessages {
			tokens = maxMessages
		} else if tokens < 1 {
			required := 1 - tokens
			time.Sleep(time.Duration(required * float64(time.Second)))
		}

		if err := c.write(data); err != nil {
			c.log("ERROR sending: %s", err)
			return
		}

		tokens--
	}
}

func (c *Client) startRecvLoop() error {
	for {
		c.conn.SetReadDeadline(time.Now().Add(c.readTimeout))
		line, err := c.reader.ReadString('\n')
		if err != nil {
			close(c.sendQueue)
			return err
		}
		c.log("> %s", line)
		c.doCallbacks(line)
	}
}

func (c *Client) doCallbacks(line string) {
	msg := NewMessage(line)
	if msg.Command == "PRIVMSG" {
		var m string
		if len(msg.Params) > 1 {
			m = msg.Params[1]
		}

		if strings.HasPrefix(m, "\u0001ACTION") {
			c.doActionCallbacks(&msg)
		} else {
			if _, cheered := msg.Tags["bits"]; cheered {
				c.doCheerCallbacks(&msg)
			} else {
				c.doChatCallbacks(&msg)
			}
		}
	} else if msg.Command == "JOIN" {
		c.doJoinCallbacks(&msg)
	} else if msg.Command == "USERNOTICE" {
		msgid := msg.Tags["msg-id"]
		if msgid == "resub" {
			c.doResubCallbacks(&msg)
		} else if msgid == "sub" {
			c.doSubscriptionCallbacks(&msg)
		}
	}
}

func (c *Client) doResubCallbacks(msg *Message) {
	c.callbackMu.Lock()
	callbacks := c.resubCallbacks
	c.callbackMu.Unlock()

	for _, cb := range callbacks {
		cb(msg.Params[0], msg.Tags, msg.Params[1])
	}
}

func (c *Client) doSubscriptionCallbacks(msg *Message) {
	c.callbackMu.Lock()
	callbacks := c.subscriptionCallbacks
	c.callbackMu.Unlock()

	for _, cb := range callbacks {
		cb(msg.Params[0], msg.Tags, msg.Params[1])
	}
}

func (c *Client) doCheerCallbacks(msg *Message) {
	c.callbackMu.Lock()
	callbacks := c.cheerCallbacks
	c.callbackMu.Unlock()

	for _, cb := range callbacks {
		cb(msg.Params[0], msg.Tags, msg.Params[1])
	}
}

func (c *Client) doActionCallbacks(msg *Message) {
	c.callbackMu.Lock()
	callbacks := c.actionCallbacks
	c.callbackMu.Unlock()

	m := msg.Params[1]
	for _, cb := range callbacks {
		cb(msg.Params[0], msg.Tags, m[7:])
	}
}

func (c *Client) doChatCallbacks(msg *Message) {
	c.callbackMu.Lock()
	callbacks := c.chatCallbacks
	c.callbackMu.Unlock()

	for _, cb := range callbacks {
		cb(msg.Params[0], msg.Tags, msg.Params[1])
	}
}

func (c *Client) doJoinCallbacks(msg *Message) {
	c.callbackMu.Lock()
	callbacks := c.joinCallbacks
	c.callbackMu.Unlock()

	for _, cb := range callbacks {
		cb(msg.Params[0], msg.Prefix.Nick)
	}
}
