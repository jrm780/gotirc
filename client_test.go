package gotirc

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

const username = "TEST_NAME"
const password = "TEST_PASS"

func createClientServer() (*Client, net.Conn) {
	var client Client
	var server net.Conn
	client.conn, server = net.Pipe()
	client.reader = bufio.NewReader(client.conn)
	client.writer = bufio.NewWriter(client.conn)
	client.doneChan = make(chan struct{})
	return &client, server
}

func TestAuthenticate(t *testing.T) {
	var wg sync.WaitGroup
	client, server := createClientServer()
	go func() {
		wg.Add(1)
		defer wg.Done()

		err := client.authenticate(username, password)
		if err != nil {
			t.Errorf("Expected 'nil', got %s", err)
		}
	}()

	in := bufio.NewReader(server)
	out := bufio.NewWriter(server)

	line, _ := in.ReadString('\n')
	if line != "PASS "+password+"\r\n" {
		t.Errorf("Expected '%s', got '%s'", "PASS "+password, line)
	}

	line, _ = in.ReadString('\n')
	if line != "NICK "+username+"\r\n" {
		t.Errorf("Expected '%s', got '%s'", "NICK "+username, line)
	}

	out.WriteString(":tmi.twitch.tv 001 " + username + " :Welcome, GLHF!\r\n")
	out.Flush()

	line, _ = in.ReadString('\n')
	if line != fmt.Sprintf("CAP REQ :%s\r\n", strings.Join(caps, " twitch.tv/")) {
		t.Errorf("Expected caps '%v', got '%s'", caps, line)
	}

	server.Close()
	wg.Wait()
}

func TestFailedAuthenticate(t *testing.T) {
	var wg sync.WaitGroup
	client, server := createClientServer()
	go func() {
		wg.Add(1)
		defer wg.Done()

		err := client.authenticate(username, password)
		if err == nil {
			t.Errorf("Expected 'error', got %s", err)
		}
	}()

	in := bufio.NewReader(server)
	out := bufio.NewWriter(server)

	in.ReadString('\n') // nick
	in.ReadString('\n') // pass

	out.WriteString(":tmi.twitch.tv XXX " + username + " :Welcome, GLHF!\r\n")
	out.Flush()

	server.Close()
	wg.Wait()
}

func TestSend(t *testing.T) {
	test := "test\n"
	client := NewClient(Options{})
	client.sendQueue = make(chan string, sendBufferSize)

	// Client not yet connected
	client.send(test)
	select {
	case <-client.sendQueue:
		t.Error("Expected empty channel")
	default:
	}

	client.connected = true
	client.send(test)
	client.send("%s", test)

	select {
	case data := <-client.sendQueue:
		if data != test {
			t.Errorf("Expected '%s', got '%s'", test, data)
		}
	default:
		t.Error("Expected nonempty channel")
	}

	select {
	case data := <-client.sendQueue:
		if data != test {
			t.Errorf("Expected '%s', got '%s'", test, data)
		}
	default:
		t.Error("Expected nonempty channel")
	}

	for i := 0; i <= sendBufferSize; i++ {
		client.send(test)
	}
	if len(client.sendQueue) != sendBufferSize {
		t.Errorf("Expected '%d', got '%d'", sendBufferSize, len(client.sendQueue))
	}
}

func TestWrite(t *testing.T) {
	expect := "test\n"
	var wg sync.WaitGroup
	client, server := createClientServer()
	go func() {
		wg.Add(1)
		defer wg.Done()

		if err := client.write(expect); err != nil {
			t.Errorf("Expected 'nil', got %s", err)
		}
		if err := client.write(expect); err == nil {
			t.Errorf("Expected 'non-nil', got nil")
		}
	}()

	in := bufio.NewReader(server)

	line, _ := in.ReadString('\n')
	if line != expect {
		t.Errorf("Expected '%s', got '%s'", expect, line)
	}

	server.Close()
	wg.Wait()
}

func TestJoin(t *testing.T) {
	channel1 := "test1"
	channel2 := "#test2"
	client := NewClient(Options{})
	client.sendQueue = make(chan string, sendBufferSize)
	client.connected = true
	client.Join(channel1)
	client.Join(channel2)

	select {
	case data := <-client.sendQueue:
		expect := "JOIN #" + channel1
		if data != expect {
			t.Errorf("Expected '%s', got '%s'", expect, data)
		}
	default:
		t.Error("Expected nonempty channel")
	}

	select {
	case data := <-client.sendQueue:
		expect := "JOIN " + channel2
		if data != expect {
			t.Errorf("Expected '%s', got '%s'", expect, data)
		}
	default:
		t.Error("Expected nonempty channel")
	}
}

func TestPart(t *testing.T) {
	channel1 := "test1"
	channel2 := "#test2"
	client := NewClient(Options{})
	client.sendQueue = make(chan string, sendBufferSize)
	client.connected = true
	client.Part(channel1)
	client.Part(channel2)

	select {
	case data := <-client.sendQueue:
		expect := "PART #" + channel1
		if data != expect {
			t.Errorf("Expected '%s', got '%s'", expect, data)
		}
	default:
		t.Error("Expected nonempty channel")
	}

	select {
	case data := <-client.sendQueue:
		expect := "PART " + channel2
		if data != expect {
			t.Errorf("Expected '%s', got '%s'", expect, data)
		}
	default:
		t.Error("Expected nonempty channel")
	}
}

func TestSendLoop(t *testing.T) {
	maxBurst := 10
	perSeconds := 2
	var wg sync.WaitGroup
	client, server := createClientServer()
	client.sendQueue = make(chan string, sendBufferSize)
	client.connected = true
	go func() {
		wg.Add(1)
		defer wg.Done()

		client.startSendLoop(float64(maxBurst), float64(perSeconds))
	}()

	in := bufio.NewReader(server)

	for i := 0; i < maxBurst*2; i++ {
		client.send("%d", i)
	}

	data := make([]string, maxBurst*2)
	recv := make([]time.Time, maxBurst*2)
	for i := 0; i < maxBurst*2; i++ {
		data[i], _ = in.ReadString('\n')
		recv[i] = time.Now()
	}

	delta := recv[len(recv)-1].Sub(recv[0])
	minTime := time.Duration(perSeconds) * time.Second
	if delta < minTime {
		t.Errorf("Expected delta > %s, got %s (%s - %s)", minTime, delta, recv[len(recv)-1], recv[0])
	}

	for i := 0; i < maxBurst*2; i++ {
		expected := fmt.Sprintf("%d\r\n", i)
		if data[i] != expected {
			t.Errorf("Expected '%s', got '%s'", expected, data[i])
		}
	}

	// If server has closed connection, the send loop should terminate
	server.Close()
	client.send("X")

	wg.Wait()
}

func TestEndRecvLoop(t *testing.T) {
	var wg sync.WaitGroup
	client, server := createClientServer()
	client.sendQueue = make(chan string, sendBufferSize)
	go func() {
		wg.Add(1)
		defer wg.Done()

		err := client.startRecvLoop()
		if err == nil {
			t.Errorf("Expected 'non-nil' error, got nil")
		}
	}()

	out := bufio.NewWriter(server)
	out.WriteString("test\r\n")
	out.Flush()

	server.Close()
	wg.Wait()
}

func TestTimeoutRecvLoop(t *testing.T) {
	var wg sync.WaitGroup
	client, server := createClientServer()
	client.sendQueue = make(chan string, sendBufferSize)
	go func() {
		wg.Add(1)
		defer wg.Done()

		client.readTimeout = 100 * time.Millisecond
		err := client.startRecvLoop()
		if err == nil {
			t.Errorf("Expected 'non-nil' error, got nil")
		}
	}()
	wg.Wait()
	server.Close()
}

func TestOnJoin(t *testing.T) {
	client := NewClient(Options{})
	expectedNick := "test_nick"
	expectedChan := "#test"
	var gotChan string
	var gotNick string
	client.OnJoin(func(channel, nick string) {
		gotChan, gotNick = channel, nick
	})
	client.doCallbacks(fmt.Sprintf(":%s!%s@%s.tmi.twitch.tv JOIN %s\r\n",
		expectedNick, expectedNick, expectedNick, expectedChan))

	if expectedNick != gotNick {
		t.Errorf("Expected '%s', got '%s'", expectedNick, gotNick)
	}
	if expectedChan != gotChan {
		t.Errorf("Expected '%s', got '%s'", expectedChan, gotChan)
	}
}

func TestOnPart(t *testing.T) {
	client := NewClient(Options{})
	expectedNick := "test_nick"
	expectedChan := "#test"
	var gotChan string
	var gotNick string
	client.OnPart(func(channel, nick string) {
		gotChan, gotNick = channel, nick
	})
	client.doCallbacks(fmt.Sprintf(":%s!%s@%s.tmi.twitch.tv PART %s\r\n",
		expectedNick, expectedNick, expectedNick, expectedChan))

	if expectedNick != gotNick {
		t.Errorf("Expected '%s', got '%s'", expectedNick, gotNick)
	}
	if expectedChan != gotChan {
		t.Errorf("Expected '%s', got '%s'", expectedChan, gotChan)
	}
}

func TestOnChat(t *testing.T) {
	client := NewClient(Options{})
	expectedChan := "#test"
	expectedTags := map[string]string{"display-name": "Test_Nick", "mod": "1"}
	expectedMsg := "Test message!"
	var gotChan string
	var gotTags map[string]string
	var gotMsg string
	client.OnChat(func(channel string, tags map[string]string, msg string) {
		gotChan = channel
		gotTags = tags
		gotMsg = msg
	})

	line := createMessage("PRIVMSG", expectedChan, expectedMsg, expectedTags)
	client.doCallbacks(line)

	if expectedMsg != gotMsg {
		t.Errorf("Expected '%s', got '%s'", expectedMsg, gotMsg)
	}
	if expectedChan != gotChan {
		t.Errorf("Expected '%s', got '%s'", expectedChan, gotChan)
	}
	for k := range expectedTags {
		if expectedTags[k] != gotTags[k] {
			t.Errorf("Expected '%s', got '%s'", expectedTags[k], gotTags[k])
		}
	}
}

func TestOnAction(t *testing.T) {
	client := NewClient(Options{})
	expectedChan := "#test"
	expectedTags := map[string]string{"display-name": "Test_Nick", "mod": "1"}
	expectedMsg := "Test message!"
	var gotChan string
	var gotTags map[string]string
	var gotMsg string
	client.OnAction(func(channel string, tags map[string]string, msg string) {
		gotChan = channel
		gotTags = tags
		gotMsg = msg
	})

	line := createMessage("PRIVMSG", expectedChan, "\u0001ACTION"+expectedMsg, expectedTags)
	client.doCallbacks(line)

	if expectedMsg != gotMsg {
		t.Errorf("Expected '%s', got '%s'", expectedMsg, gotMsg)
	}
	if expectedChan != gotChan {
		t.Errorf("Expected '%s', got '%s'", expectedChan, gotChan)
	}
	for k := range expectedTags {
		if expectedTags[k] != gotTags[k] {
			t.Errorf("Expected '%s', got '%s'", expectedTags[k], gotTags[k])
		}
	}
}

func TestOnCheer(t *testing.T) {
	client := NewClient(Options{})
	expectedChan := "#test"
	expectedTags := map[string]string{"display-name": "Test_Nick", "mod": "1", "bits": "100"}
	expectedMsg := "Test message!"
	var gotChan string
	var gotTags map[string]string
	var gotMsg string
	client.OnCheer(func(channel string, tags map[string]string, msg string) {
		gotChan = channel
		gotTags = tags
		gotMsg = msg
	})

	line := createMessage("PRIVMSG", expectedChan, expectedMsg, expectedTags)
	client.doCallbacks(line)

	if expectedMsg != gotMsg {
		t.Errorf("Expected '%s', got '%s'", expectedMsg, gotMsg)
	}
	if expectedChan != gotChan {
		t.Errorf("Expected '%s', got '%s'", expectedChan, gotChan)
	}
	for k := range expectedTags {
		if expectedTags[k] != gotTags[k] {
			t.Errorf("Expected '%s', got '%s'", expectedTags[k], gotTags[k])
		}
	}
}

func TestOnResub(t *testing.T) {
	client := NewClient(Options{})
	expectedChan := "#test"
	expectedTags := map[string]string{"msg-id": "resub", "msg-param-months": "6", "msg-param-sub-plan": "Prime"}
	expectedMsg := "Test message!"
	var gotChan string
	var gotTags map[string]string
	var gotMsg string
	client.OnResub(func(channel string, tags map[string]string, msg string) {
		gotChan = channel
		gotTags = tags
		gotMsg = msg
	})

	line := createMessage("USERNOTICE", expectedChan, expectedMsg, expectedTags)
	client.doCallbacks(line)

	if expectedMsg != gotMsg {
		t.Errorf("Expected '%s', got '%s'", expectedMsg, gotMsg)
	}
	if expectedChan != gotChan {
		t.Errorf("Expected '%s', got '%s'", expectedChan, gotChan)
	}
	for k := range expectedTags {
		if expectedTags[k] != gotTags[k] {
			t.Errorf("Expected '%s', got '%s'", expectedTags[k], gotTags[k])
		}
	}
}

func TestOnSubscription(t *testing.T) {
	client := NewClient(Options{})
	expectedChan := "#test"
	expectedTags := map[string]string{"msg-id": "sub", "msg-param-sub-plan": "Prime"}
	expectedMsg := "Test message!"
	var gotChan string
	var gotTags map[string]string
	var gotMsg string
	client.OnSubscription(func(channel string, tags map[string]string, msg string) {
		gotChan = channel
		gotTags = tags
		gotMsg = msg
	})

	line := createMessage("USERNOTICE", expectedChan, expectedMsg, expectedTags)
	client.doCallbacks(line)

	if expectedMsg != gotMsg {
		t.Errorf("Expected '%s', got '%s'", expectedMsg, gotMsg)
	}
	if expectedChan != gotChan {
		t.Errorf("Expected '%s', got '%s'", expectedChan, gotChan)
	}
	for k := range expectedTags {
		if expectedTags[k] != gotTags[k] {
			t.Errorf("Expected '%s', got '%s'", expectedTags[k], gotTags[k])
		}
	}
}

func TestSay(t *testing.T) {
	client := NewClient(Options{})
	client.sendQueue = make(chan string, sendBufferSize)
	client.connected = true

	channel := "channel1"
	msg := "This is a test"
	client.Say(channel, msg)
	client.Say("#"+channel, msg)

	select {
	case data := <-client.sendQueue:
		expect := fmt.Sprintf("PRIVMSG #%s :%s", channel, msg)
		if data != expect {
			t.Errorf("Expected '%s', got '%s'", expect, data)
		}
	default:
		t.Error("Expected nonempty channel")
	}

	select {
	case data := <-client.sendQueue:
		expect := fmt.Sprintf("PRIVMSG #%s :%s", channel, msg)
		if data != expect {
			t.Errorf("Expected '%s', got '%s'", expect, data)
		}
	default:
		t.Error("Expected nonempty channel")
	}
}

func TestWhisper(t *testing.T) {
	client := NewClient(Options{})
	client.sendQueue = make(chan string, sendBufferSize)
	client.connected = true

	nick := "testnick"
	msg := "This is a test"
	client.Whisper(nick, msg)

	select {
	case data := <-client.sendQueue:
		expect := fmt.Sprintf("PRIVMSG #jtv :/w %s %s", nick, msg)
		if data != expect {
			t.Errorf("Expected '%s', got '%s'", expect, data)
		}
	default:
		t.Error("Expected nonempty channel")
	}
}

func TestCloseConnection(t *testing.T) {
	var wg sync.WaitGroup
	client, server := createClientServer()
	client.options.Channels = []string{"test_channel1"}
	go func() {
		wg.Add(1)
		defer wg.Done()

		// Client not yet connected
		if client.Connected() {
			t.Error("Expected 'false', got 'true'")
		}

		// First-time connection
		_, err := client.doConnect(func() (net.Conn, error) {
			return client.conn, nil
		})
		if err != nil {
			t.Errorf("Expected 'nil' error, got %s", err)
		}

		// Already connected, should return error
		_, err = client.doConnect(func() (net.Conn, error) {
			return client.conn, nil
		})
		if err == nil {
			t.Errorf("Expected 'non-nil' error, got nil")
		}

		client.readTimeout = 500 * time.Millisecond
		err = client.doPostConnect("test", "test", client.conn, 10, 2)
		if err == nil {
			t.Errorf("Expected 'non-nil' error, got nil")
		}
	}()

	in := bufio.NewReader(server)
	out := bufio.NewWriter(server)

	line, _ := in.ReadString('\n') // PASS
	line, _ = in.ReadString('\n')  // NICK
	out.WriteString(":tmi.twitch.tv 001 " + username + " :Welcome, GLHF!\r\n")
	out.Flush()

	line, _ = in.ReadString('\n')
	if line != fmt.Sprintf("CAP REQ :%s\r\n", strings.Join(caps, " twitch.tv/")) {
		t.Errorf("Expected caps '%v', got '%s'", caps, line)
	}

	if !client.Connected() {
		t.Error("Expected 'true', got 'false'")
	}

	line, _ = in.ReadString('\n') // JOIN

	server.Close()
	wg.Wait()

	// Should be disconnected after server closed connection
	if client.Connected() {
		t.Error("Expected 'false', got 'true'")
	}
}

func TestDisconnect(t *testing.T) {
	var wg sync.WaitGroup
	client, server := createClientServer()
	client.options.Channels = []string{"test_channel1"}
	go func() {
		wg.Add(1)
		defer wg.Done()

		// Client not yet connected
		if client.Connected() {
			t.Error("Expected 'false', got 'true'")
		}

		_, err := client.doConnect(func() (net.Conn, error) {
			return client.conn, nil
		})
		if err != nil {
			t.Errorf("Expected 'nil' error, got %s", err)
		}

		client.readTimeout = 500 * time.Millisecond
		err = client.doPostConnect("test", "test", client.conn, 10, 2)
		if err == nil {
			t.Errorf("Expected 'non-nil' error, got nil")
		}
	}()

	in := bufio.NewReader(server)
	out := bufio.NewWriter(server)

	line, _ := in.ReadString('\n') // PASS
	line, _ = in.ReadString('\n')  // NICK
	out.WriteString(":tmi.twitch.tv 001 " + username + " :Welcome, GLHF!\r\n")
	out.Flush()

	line, _ = in.ReadString('\n')
	if line != fmt.Sprintf("CAP REQ :%s\r\n", strings.Join(caps, " twitch.tv/")) {
		t.Errorf("Expected caps '%v', got '%s'", caps, line)
	}

	if !client.Connected() {
		t.Error("Expected 'true', got 'false'")
	}

	line, _ = in.ReadString('\n') // JOIN

	client.Disconnect()
	wg.Wait()

	// Should be disconnected after server closed connection
	if client.Connected() {
		t.Error("Expected 'false', got 'true'")
	}
}

func TestOnPing(t *testing.T) {
	client := NewClient(Options{})
	client.sendQueue = make(chan string, 1)
	client.connected = true
	client.doCallbacks("PING :tmi.twitch.tv\r\n")

	line := <-client.sendQueue
	expect := "PONG :tmi.twitch.tv"
	if line != expect {
		t.Errorf("Expected '%s', got '%s'", expect, line)
	}
}

func TestLog(t *testing.T) {
	client := NewClient(Options{})
	in, out := net.Pipe()
	r := bufio.NewReader(in)

	log.SetOutput(out)

	go func() {
		client.log("A") // Should not be logged
		client.options.Debug = true
		client.log("B\n") // Should be logged
	}()

	line, _ := r.ReadString('\n')
	if line[len(line)-3:] != " B\n" {
		t.Errorf("Expected ' B\n', got '%s'", line[len(line)-3:])
	}

	in.Close()
	out.Close()
}

func createMessage(msgType, channel, msg string, tags map[string]string) string {
	var data bytes.Buffer
	data.WriteRune('@')
	size := 1
	for k, v := range tags {
		n, _ := data.WriteString(k)
		size += n
		n, _ = data.WriteRune('=')
		size += n
		n, _ = data.WriteString(v)
		size += n
		n, _ = data.WriteRune(';')
		size += n
	}
	size--
	data.Truncate(size)
	data.WriteString(" :x!x@x.tmi.twitch.tv ")
	data.WriteString(msgType)
	data.WriteRune(' ')
	data.WriteString(channel)
	data.WriteString(" :")
	data.WriteString(msg)
	data.WriteString("\r\n")
	return data.String()
}
