// Package gotirc contains functions for connecting to Twitch.tv chat via IRC
package gotirc

import "strings"

// Message holds data received from the server
type Message struct {
	Raw     string
	Prefix  Prefix
	Command string
	Params  []string
	Tags    map[string]string
}

// Prefix is a component of an IRC Message
type Prefix struct {
	Raw  string
	Nick string
	User string
	Host string
}

// NewPrefix instantiates a Prefix from a raw prefix string (e.g., <user>!<user>@<user>.tmi.twitch.tv)
func NewPrefix(raw string) Prefix {
	p := Prefix{Raw: raw}
	userIndex := strings.IndexRune(raw, '!')
	hostIndex := strings.IndexRune(raw, '@')
	if hostIndex < 0 && userIndex < 0 {
		p.Nick = raw
		return p
	}

	if userIndex >= 0 {
		p.Nick = raw[:userIndex]
		if hostIndex >= 0 {
			p.User = raw[userIndex+1 : hostIndex]
		} else {
			p.User = raw[userIndex+1:]
		}
	}

	if hostIndex >= 0 {
		p.Host = raw[hostIndex+1:]
		if userIndex < 0 {
			p.Nick = raw[:hostIndex]
		}
	}

	return p
}

// NewMessage parses received IRC data into a Message
func NewMessage(message string) Message {
	msg := Message{
		Raw:  strings.TrimSpace(message),
		Tags: make(map[string]string),
	}

	pos := 0
	nextSpace := 0

	// Parse tags (optional)
	if strings.HasPrefix(msg.Raw, "@") {
		nextSpace = strings.Index(msg.Raw, " ")
		if nextSpace < 0 {
			return msg // error
		}

		rawTags := strings.Split(msg.Raw[1:nextSpace], ";")
		for _, t := range rawTags {
			tag := strings.Split(t, "=")
			if len(tag) < 2 {
				msg.Tags[tag[0]] = ""
			} else {
				msg.Tags[tag[0]] = tag[1]
			}
		}
		pos = nextSpace + 1
	}

	// Parse prefix (optional)
	for msg.Raw[pos] == ' ' {
		pos++
	}
	if msg.Raw[pos] == ':' {
		nextSpace = strings.Index(msg.Raw[pos:], " ")
		if nextSpace < 0 {
			return msg // error
		}
		nextSpace += pos
		msg.Prefix = NewPrefix(msg.Raw[pos+1 : nextSpace])
		pos = nextSpace + 1
	}

	// Parse command and params
	rawLen := len(msg.Raw)
	for msg.Raw[pos] == ' ' {
		pos++
	}
	nextSpace = strings.Index(msg.Raw[pos:], " ")
	if nextSpace < 0 {
		if rawLen > pos {
			msg.Command = msg.Raw[pos:]
		}
	} else {
		nextSpace += pos
		msg.Command = msg.Raw[pos:nextSpace]
		pos = nextSpace + 1
		for msg.Raw[pos] == ' ' {
			pos++
		}
		for pos < rawLen {
			nextSpace = strings.Index(msg.Raw[pos:], " ")
			if msg.Raw[pos] == ':' {
				msg.Params = append(msg.Params, msg.Raw[pos+1:])
				return msg
			}
			if nextSpace != -1 {
				nextSpace += pos
				msg.Params = append(msg.Params, msg.Raw[pos:nextSpace])
				pos = nextSpace + 1
				for msg.Raw[pos] == ' ' {
					pos++
				}
			} else {
				msg.Params = append(msg.Params, msg.Raw[pos:])
				return msg
			}
		}
	}
	return msg
}
