package gotirc

import "testing"

/* Twitch uses a variation of the RFC 1459 message format:
 * [':' <prefix> <SPACE> ] <command> <params> <crlf>
 * prefix = <servername> | <nick> [ '!' <user> ] [ '@' <host> ]
 * command = <letter> { <letter> } | <number> <number> <number>
 * <params> = <SPACE> [ ':' <trailing> | <middle> <params> ]
 * <middle> = <Any *non-empty* sequence of octets not including SPACE
 *            or NUL or CR or LF, the first of which may not be ':'>
 * <trailing> = <Any, possibly *empty*, sequence of octets not including
 *              NUL or CR or LF>
 */

func TestPrefix(t *testing.T) {
	prefix := NewPrefix("nick123")
	if prefix.Nick != "nick123" {
		t.Errorf("Expected 'nick123', got '%s'", prefix.Nick)
	}
	if prefix.Raw != "nick123" {
		t.Errorf("Expected 'nick123', got '%s'", prefix.Raw)
	}

	prefix = NewPrefix("nick123!user123")
	if prefix.Nick != "nick123" {
		t.Errorf("Expected 'nick123', got '%s'", prefix.Nick)
	}
	if prefix.Raw != "nick123!user123" {
		t.Errorf("Expected 'nick123@host.com', got '%s'", prefix.Raw)
	}
	if prefix.User != "user123" {
		t.Errorf("Expected 'user123', got '%s'", prefix.User)
	}

	prefix = NewPrefix("nick123@host.com")
	if prefix.Nick != "nick123" {
		t.Errorf("Expected 'nick123', got '%s'", prefix.Nick)
	}
	if prefix.Raw != "nick123@host.com" {
		t.Errorf("Expected 'nick123@host.com', got '%s'", prefix.Raw)
	}
	if prefix.Host != "host.com" {
		t.Errorf("Expected 'host.com', got '%s'", prefix.Host)
	}

	prefix = NewPrefix("nick123!user123@host.com")
	if prefix.Nick != "nick123" {
		t.Errorf("Expected 'nick123', got '%s'", prefix.Nick)
	}
	if prefix.Raw != "nick123!user123@host.com" {
		t.Errorf("Expected 'nick123!user123@host.com', got '%s'", prefix.Raw)
	}
	if prefix.Host != "host.com" {
		t.Errorf("Expected 'host.com', got '%s'", prefix.Host)
	}
	if prefix.User != "user123" {
		t.Errorf("Expected 'user123', got '%s'", prefix.User)
	}

	prefix = NewPrefix("tmi.twitch.tv")
	if prefix.Nick != "tmi.twitch.tv" {
		t.Errorf("Expected 'tmi.twitch.tv', got '%s'", prefix.Nick)
	}
	if prefix.Raw != "tmi.twitch.tv" {
		t.Errorf("Expected 'tmi.twitch.tv', got '%s'", prefix.Raw)
	}
	if prefix.Host != "" {
		t.Errorf("Expected '', got '%s'", prefix.Host)
	}
	if prefix.User != "" {
		t.Errorf("Expected '', got '%s'", prefix.User)
	}
}

func TestConnectMessage(t *testing.T) {
	raw := `:tmi.twitch.tv 002 user123 :Your host is tmi.twitch.tv`
	msg := NewMessage(raw)
	if msg.Prefix.Raw != "tmi.twitch.tv" {
		t.Errorf("Expected 'tmi.twitch.tv', got %s", msg.Prefix.Raw)
	}
	if msg.Command != "002" {
		t.Errorf("Expected '002', got %s", msg.Command)
	}
	if len(msg.Params) != 2 {
		t.Errorf("Expected '2' parameters, got %d", len(msg.Params))
	}
	if msg.Params[0] != "user123" {
		t.Errorf("Expected 'user123' parameters, got %s", msg.Params[0])
	}
	if msg.Params[1] != "Your host is tmi.twitch.tv" {
		t.Errorf("Expected 'Your host is tmi.twitch.tv' parameters, got %s", msg.Params[1])
	}
}

func TestJoinMessage(t *testing.T) {
	raw := `:nick123!nick123@nick123.tmi.twitch.tv JOIN #channel`
	msg := NewMessage(raw)
	if msg.Prefix.Raw != "nick123!nick123@nick123.tmi.twitch.tv" {
		t.Errorf("Expected 'nick123!nick123@nick123.tmi.twitch.tv', got %s", msg.Prefix.Raw)
	}
	if msg.Command != "JOIN" {
		t.Errorf("Expected 'JOIN', got '%s'", msg.Command)
	}
	if len(msg.Params) != 1 {
		t.Errorf("Expected '1' parameter, got '%d'", len(msg.Params))
	}
	if msg.Params[0] != "#channel" {
		t.Errorf("Expected '#channel', got '%s'", msg.Params[0])
	}
}

func TestPrivMessageTags(t *testing.T) {
	raw := `@badges=staff/1,bits/1000;color=#ffffff;display-name=Nick123;emote-sets=0,33,50;mod;ban-reason=Follow\sthe\srules  :nick123!nick123@nick123.tmi.twitch.tv  PRIVMSG  #channel  :This is a sample message `
	msg := NewMessage(raw)
	if msg.Prefix.Raw != "nick123!nick123@nick123.tmi.twitch.tv" {
		t.Errorf("Expected 'nick123!nick123@nick123.tmi.twitch.tv', got %s", msg.Prefix.Raw)
	}
	if msg.Command != "PRIVMSG" {
		t.Errorf("Expected 'PRIVMSG', got '%s'", msg.Command)
	}
	if len(msg.Params) != 2 {
		t.Errorf("Expected '2' parameter, got '%d'", len(msg.Params))
	}
	if msg.Params[0] != "#channel" {
		t.Errorf("Expected '#channel', got '%s'", msg.Params[0])
	}
	if msg.Params[1] != "This is a sample message" {
		t.Errorf("Expected 'This is a sample message', got '%s'", msg.Params[0])
	}
	if len(msg.Tags) != 6 {
		t.Errorf("Expected '6' tags, got '%d'", len(msg.Tags))
	}
	if msg.Tags["badges"] != "staff/1,bits/1000" {
		t.Errorf("Expected 'staff/1,bits/1000', got '%s'", msg.Tags["badges"])
	}
	if msg.Tags["color"] != "#ffffff" {
		t.Errorf("Expected '#ffffff', got '%s'", msg.Tags["color"])
	}
	if msg.Tags["display-name"] != "Nick123" {
		t.Errorf("Expected 'Nick123', got '%s'", msg.Tags["display-name"])
	}
	if msg.Tags["emote-sets"] != "0,33,50" {
		t.Errorf("Expected '0,33,50', got '%s'", msg.Tags["emote-sets"])
	}
	if msg.Tags["mod"] != "" {
		t.Errorf("Expected '', got '%s'", msg.Tags["badges"])
	}
	if msg.Tags["ban-reason"] != `Follow\sthe\srules` {
		t.Errorf(`Expected 'Follow\sthe\srules', got '%s'`, msg.Tags["ban-reason"])
	}
}

func TestTagsOnly(t *testing.T) {
	raw := `@this=is\sbroken:tmi.witch.tv`
	msg := NewMessage(raw)
	if msg.Command != "" {
		t.Errorf(`Expected '', got '%s'`, msg.Command)
	}
	if len(msg.Params) != 0 {
		t.Errorf(`Expected 0 params, got %d`, len(msg.Params))
	}
	if len(msg.Tags) != 0 {
		t.Errorf(`Expected 0 tags, got %d`, len(msg.Tags))
	}
	if msg.Raw != raw {
		t.Errorf(`Expcted %s, got %s`, raw, msg.Raw)
	}
}

func TestTagsPrefixOnly(t *testing.T) {
	raw := `@this=is\sbroken:tmi.witch.tv :tmi.witch.tv`
	msg := NewMessage(raw)
	if msg.Command != "" {
		t.Errorf(`Expected '', got '%s'`, msg.Command)
	}
	if len(msg.Params) != 0 {
		t.Errorf(`Expected 0 params, got %d`, len(msg.Params))
	}
	if len(msg.Tags) != 1 {
		t.Errorf(`Expected 1 tags, got %d`, len(msg.Tags))
	}
	if msg.Raw != raw {
		t.Errorf(`Expcted %s, got %s`, raw, msg.Raw)
	}
}

func TestNoParams(t *testing.T) {
	raw := `:tmi.twitch.tv 002`
	msg := NewMessage(raw)
	if msg.Command != "002" {
		t.Errorf(`Expected '002', got %s`, msg.Command)
	}
}

func TestCommandOnly(t *testing.T) {
	raw := `asdf`
	msg := NewMessage(raw)
	if msg.Command != "asdf" {
		t.Errorf(`Expected 'asdf', got '%s'`, msg.Command)
	}
	if len(msg.Params) != 0 {
		t.Errorf(`Expected 0 params, got %d`, len(msg.Params))
	}
	if len(msg.Tags) != 0 {
		t.Errorf(`Expected 0 tags, got %d`, len(msg.Tags))
	}
	if msg.Raw != raw {
		t.Errorf(`Expcted %s, got %s`, raw, msg.Raw)
	}
}
