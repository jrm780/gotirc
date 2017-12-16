# gotirc
![](https://travis-ci.org/jrm780/gotirc.svg?branch=master) [![](https://coveralls.io/repos/github/jrm780/gotirc/badge.svg?branch=master)](https://coveralls.io/github/jrm780/gotirc?branch=master)

### A Twitch.tv IRC library written in Go

[Twitch chat](https://dev.twitch.tv/docs/irc) uses a protocol built on TCP that is roughly based on Internet Relay Chat (IRC) RFC 1459. This package provides convenience functionality for implementing software that requires Twitch chat interaction (e.g., bots, statistics, etc.).

#### Client
A `gotirc.Client` is used to connect to Twitch chat. Callbacks can be passed to the Client to perform actions when particular events occur.

```go
    package main
    
    import (
        "log"
        "github.com/jrm780/gotirc"
    )
    
    func main() {
        options := gotirc.Options{
            Host:     "irc.chat.twitch.tv",
            Port:     6667,
            Channels: []string{"#twitch"},
        }

        client := gotirc.NewClient(options)
        
        // Whenever someone sends a message, log it
        client.OnChat(func(channel string, tags map[string]string, msg string) {
            log.Printf("[%s]: %s", tags["display-name"], msg)
        })
        
        // Connect and authenticate with the given username and oauth token
        client.Connect("justinfan1337", "abc123")
    }
```

`Client.Connect(nick, pass)` runs until the client is disconnected from the server. It's easy to implement automatic reconnecting by using a for-loop:

```go
    client := gotirc.NewClient(options)
    for {
        err := client.Connect("justinfan1337", "abc123")
        fmt.Printf("Disconnected: %s. Reconnecting in 5 seconds...\n", err)
        time.Sleep(5*time.Second)
    }
```

#### Currently Implemented Callbacks
* **OnAction(**_func(channel string, tags map[string]string, msg string)_**)**
  * Adds an event callback for action (e.g., /me) messages
* **OnChat(**_func(channel string, tags map[string]string, msg string)_**)**
  * Adds an event callback for when a user sends a message in a channel
* **OnResub(**_func(channel string, tags map[string]string, msg string)_**)**
  * Adds an event callback for when a user resubs to a channel
* **OnSubscription(**_func(channel string, tags map[string]string, msg string)_**)**
  * Adds an event callback for when a user subscribes to a channel
* **OnCheer(**_func(channel string, tags map[string]string, msg string)_**)**
  * Adds an event callback for when a user cheers bits in a channel
* **OnJoin(**_func(channel, username string)_**)**
  * Adds an event callback for when a user joins a channel
  
Tags are metadata associated with the message and include information such as the user's display-name and chat color. Twitch may change the tags at any time, so it's best to refer to [their documentation](https://dev.twitch.tv/docs/irc#privmsg-twitch-tags) to determine which data is available.
