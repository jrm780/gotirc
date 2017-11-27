# gotirc
A Twitch.tv IRC library written in Go

Message parsing (as indicated to be roughly based on RFC 1459 by [Twitch](https://dev.twitch.tv/docs/irc) has been implemented in message.go.

```go
// Parse a message receieved from Twitch.tv IRC server
// data = @color=#ffffff;display-name=Nick123 :nick123!nick123@nick123.tmi.twitch.tv PRIVMSG #channel :This is a sample message
msg := gotirc.NewMessage(data)

if msg.Command == "PRIVMSG" {
    // "Nick123 said This is a sample message"
    fmt.Printf("%s said %s", msg.Tags["display-name"], msg.Params[1])
}
```

Client/connection handling to come shortly.