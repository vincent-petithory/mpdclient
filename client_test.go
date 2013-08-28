package mpdfav

import (
	"testing"
	"time"
	"fmt"
)

const (
	mpdHost = "localhost"
	mpdPort = 6600
)

type idleTestCase struct {
	Name string
	Subsystems []string
	ExpectedSubsystemsNotifications []string
}

func TestStatus(t *testing.T) {
	mpdc, err := Connect(mpdHost, mpdPort)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		fmt.Println("closing")
		mpdc.Close()
		fmt.Println("closed")
	}()
	info, err := mpdc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if info == nil {
		t.Fatalf("Unexpected nil value")
	}
	fmt.Println(info)
}

// TestUnexistingSticketGet tests that StickerGet
// returns an empty string and no error
// when a sticker is not found
func TestUnexistingSticketGet(t *testing.T) {
	mpdc, err := Connect(mpdHost, mpdPort)
	if err != nil {
		t.Fatal(err)
	}
	defer mpdc.Close()
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
	if err != nil {
		t.Fatal(err)
	}
	defer mpdc.Close()
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
	if err != nil {
		t.Fatal(err)
	}
	defer mpdc.Close()
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
	if err != nil {
		t.Fatal(err)
	}
	defer mpdc.Close()
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
			t.Error(err)
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
			t.Errorf("%q != %q", channelMessage, expectedMsgs[i])
		}
	}

	err = mpdc.Unsubscribe(channel)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSimpleIdleMode(t *testing.T) {
	mpdc, err := Connect(mpdHost, mpdPort)
	if err != nil {
		t.Fatal(err)
	}
	defer mpdc.Close()

	done := make(chan struct{})
	go func() {
		v := <-mpdc.Idle("subscription")
		if v != "subscription" {
			t.Fatalf("Expected idle event %s, got %s", "subscription", v)
		}
		v = <-mpdc.Idle("message")
		if v != "message" {
			t.Fatalf("Expected idle event %s, got %s", "message", v)
		}
		close(done)
	}()
	err = mpdc.Subscribe("whatever")
	if err != nil {
		t.Fatal(err)
	}
	err = mpdc.SendMessage("whatever", "heya")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("waiting")
	<-done
	fmt.Println("done")
}

func TestIdleModeSequence(t *testing.T) {
	mpdc, err := Connect(mpdHost, mpdPort)
	if err != nil {
		t.Fatal(err)
	}
	defer mpdc.Close()

	const channelName = "test-channel"

	var idleTests = []idleTestCase{
		{"Idle 1", []string{"subscription"},  []string{"subscription", "subscription"}},
		{"Idle 2", []string{"subscription", "message"},  []string{"subscription", "message", "subscription"}},
		{"Idle 3", []string{"message"},  []string{"message"}},
	}

	idleTestsCompletions := make(chan idleTestCase)

	for _, idleTest := range idleTests {
		go func(idleTest idleTestCase) {
			for _, expectedSubsystem := range idleTest.ExpectedSubsystemsNotifications {
				subsystem := <-mpdc.Idle(idleTest.Subsystems...)
				if subsystem != expectedSubsystem {
					t.Errorf("%s: expected subsystem %s, got %s", idleTest.Name, expectedSubsystem, subsystem)
				} else {
					fmt.Println(idleTest.Name, " got", subsystem)
				}
			}
			idleTestsCompletions <- idleTest
		}(idleTest)
	}

	err = mpdc.Subscribe(channelName)
	if err != nil {
		t.Fatal(err)
	}
	err = mpdc.SendMessage(channelName, "hello MPD")
	if err != nil {
		t.Fatal(err)
	}
	err = mpdc.Unsubscribe(channelName)
	if err != nil {
		t.Fatal(err)
	}

	timeout := time.After(10*time.Second)
	n := len(idleTests)
	for n > 0 {
		if n == 0 {
			break
		}
		select {
		case <-idleTestsCompletions:
			n--
		case <-timeout:
			if n != 0 {
				t.Fatalf("Not all idle events were received. Expected %d, got %d", len(idleTests), len(idleTests) - n)
			}

		}
	}

}

