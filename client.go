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

type Client struct {
	options Options

	sendQueue   chan string
	recvChannel chan Message
	reader      *bufio.Reader
	writer      *bufio.Writer

	conn net.Conn

	callbackMu            sync.Mutex
	chatCallbacks         []func(channel string, tags map[string]string, msg string)
	resubCallbacks        []func(channel string, tags map[string]string, msg string)
	subscriptionCallbacks []func(channel string, tags map[string]string, msg string)
	cheerCallbacks        []func(channel string, tags map[string]string, msg string)
}

// NewClient returns a new Client
func NewClient(o Options) *Client {
	return &Client{
		options:   o,
		sendQueue: make(chan string, sendBufferSize),
	}
}

func (c *Client) Connect(nick string, pass string) error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", c.options.Host, c.options.Port))
	if err != nil {
		return err
	}

	err = c.doPostConnect(nick, pass, conn)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) doPostConnect(nick, pass string, conn net.Conn) error {
	c.reader = bufio.NewReader(conn)
	c.writer = bufio.NewWriter(conn)

	if err := c.authenticate(nick, pass); err != nil {
		return err
	}

	for _, channel := range c.options.Channels {
		c.Join(channel)
	}

	go c.startSendLoop()

	return nil
}

func (c *Client) OnChat(callback func(channel string, tags map[string]string, msg string)) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.chatCallbacks = append(c.chatCallbacks, callback)
}

func (c *Client) OnResub(callback func(channel string, tags map[string]string, msg string)) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.resubCallbacks = append(c.resubCallbacks, callback)
}

func (c *Client) OnSubscription(callback func(channel string, tags map[string]string, msg string)) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.subscriptionCallbacks = append(c.subscriptionCallbacks, callback)
}

func (c *Client) OnCheer(callback func(channel string, tags map[string]string, msg string)) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.cheerCallbacks = append(c.cheerCallbacks, callback)
}

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

func (c *Client) startSendLoop() {
	maxMessages := 19.0
	perSeconds := 30.0
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
	}
}
