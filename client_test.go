package mpdfav

import (
	"testing"
)

const (
	mpdHost = "localhost"
	mpdPort = 6600
)

// TestUnexistingSticketGet tests that StickerGet
// returns an empty string and no error
// when a sticker is not found
func TestUnexistingSticketGet(t *testing.T) {
	mpdc, err := Connect(mpdHost, mpdPort)
	defer mpdc.Close()
	if err != nil {
		t.Fatal(err)
	}
	value, err := mpdc.StickerGet(
		"song",
		"does-not-exist.mp3",
		"playcount",
	)
	if err == nil {
		t.Fatal("Found an unexisting song")
	}
	if len(value) != 0 {
		t.Fatalf("Return string value is not empty")
	}
}

// TestExistingSticketGet tests that StickerGet
// returns a non-empty string and no error
// when a sticker is found
func TestExistingSticketGet(t *testing.T) {
	mpdc, err := Connect(mpdHost, mpdPort)
	defer mpdc.Close()
	if err != nil {
		t.Fatal(err)
	}
	existingSticketGet(t, mpdc)
	existingSticketGet(t, mpdc)
}

func existingSticketGet(t *testing.T, mpdc *MPDClient) {
	value, err := mpdc.StickerGet(
		"song",
		"RadioFlux/FREQUENCE3 - www.frequence3.fr - It's only HITS live from Paris France ! - French Webradio/Basto - Stormchaser.mp3",
		"playcount",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(value) == 0 {
		t.Fatalf("Return string value is empty")
	}
}

// TestSubscribeSimple checks it's fine to subscribe
// then unsubscribe a channel
func TestSubscribeUnsubscribeSimple(t *testing.T) {
	mpdc, err := Connect(mpdHost, mpdPort)
	defer mpdc.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = mpdc.Subscribe("ratings")
	if err != nil {
		t.Fatal(err)
	}
	err = mpdc.Unsubscribe("ratings")
	if err != nil {
		t.Fatal(err)
	}
}

// TestSendReadMessage tests we can send then
// read a message sent on a channel
func TestSendReadMessage(t *testing.T) {
	mpdc, err := Connect(mpdHost, mpdPort)
	defer mpdc.Close()
	if err != nil {
		t.Fatal(err)
	}
	const channel = "test-channel"

	err = mpdc.Subscribe(channel)
	if err != nil {
		t.Fatal(err)
	}

	expectedMsgs := []ChannelMessage{
		ChannelMessage{channel, "first message"},
		ChannelMessage{channel, "second message"},
	}
	for _, channelMessage := range expectedMsgs {
		err = mpdc.SendMessage(channelMessage.Channel, channelMessage.Message)
		if err != nil {
			t.Fatal(err)
		}
	}

	msgs, err := mpdc.ReadMessages()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != len(expectedMsgs) {
		t.Fatalf("Expected %d messages, got %d", len(expectedMsgs), len(msgs))
	}
	for i, channelMessage := range msgs {
		if channelMessage != expectedMsgs[i] {
			t.Fatalf("%q != %q", channelMessage, expectedMsgs[i])
		}
	}

	err = mpdc.Unsubscribe(channel)
	if err != nil {
		t.Fatal(err)
	}
}
