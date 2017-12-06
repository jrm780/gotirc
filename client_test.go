package gotirc

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
)

const username = "TEST_NAME"
const password = "TEST_PASS"

func TestConnect(t *testing.T) {
	client, server := net.Pipe()
	var wg sync.WaitGroup

	go func() {
		wg.Add(1)
		defer wg.Done()

		c := NewClient(Options{})
		err := c.doPostConnect(username, password, client)
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

func TestFailedConnect(t *testing.T) {
	client, server := net.Pipe()
	var wg sync.WaitGroup

	go func() {
		wg.Add(1)
		defer wg.Done()

		c := NewClient(Options{})
		err := c.doPostConnect(username, password, client)
		if err == nil {
			t.Errorf("Expected 'non-nil error', got %s", err)
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

func TestJoin(t *testing.T) {
	client, server := net.Pipe()
	var wg sync.WaitGroup

	go func() {
		wg.Add(1)
		defer wg.Done()

		c := NewClient(Options{Channels: []string{"test_channel1", "test_channel2"}})
		err := c.doPostConnect(username, password, client)
		if err != nil {
			t.Errorf("Expected 'nil', got %s", err)
		}
	}()

	in := bufio.NewReader(server)
	out := bufio.NewWriter(server)
	doAuthHandshake(in, out)

	line, _ := in.ReadString('\n')
	if line != "JOIN #test_channel1\r\n" {
		t.Errorf("Expected 'JOIN #test_channel1\r\n', got %s", line)
	}
	// out.WriteString(":x!x@x.tmi.twitch.tv JOIN #test_channel1\r\n")
	// out.Flush()

	line, _ = in.ReadString('\n')
	if line != "JOIN #test_channel2\r\n" {
		t.Errorf("Expected 'JOIN #test_channel2\r\n', got %s", line)
	}
	// out.WriteString(":x!x@x.tmi.twitch.tv JOIN #test_channel2\r\n")
	// out.Flush()

	server.Close()
	wg.Wait()
}

func doAuthHandshake(in *bufio.Reader, out *bufio.Writer) {
	in.ReadString('\n') // nick
	in.ReadString('\n') // pass
	out.WriteString(":tmi.twitch.tv 001 " + username + " :Welcome, GLHF!\r\n")
	out.Flush()
	in.ReadString('\n') // caps
}
