package mpdclient

import (
	"errors"
	"fmt"
	"regexp"
	"sync"
	"testing"
	"time"
)

const (
	mpdHost = "localhost"
	mpdPort = 6600
)

type idleTestCase struct {
	Name                            string
	Subsystems                      []string
	ExpectedSubsystemsNotifications []string
}

type regexpTestCase struct {
	Input          string
	ExpectedGroups []string
}

func (test *regexpTestCase) Validate(regexp *regexp.Regexp) error {
	if match := regexp.FindStringSubmatch(test.Input); match == nil {
		return errors.New(fmt.Sprintf("Regexp didn't match against %s", test.Input))
	} else {
		groups := match[1:]
		if len(groups) == len(test.ExpectedGroups) {
			for i, g := range groups {
				if g != test.ExpectedGroups[i] {
					break
				}
			}
			return nil
		}
		return errors.New(fmt.Sprintf("Expected %q groups, got %q", test.ExpectedGroups, match[1:]))
	}
}

func TestStatus(t *testing.T) {
	mpdc, err := Connect(mpdHost, mpdPort)
	if err != nil {
		t.Fatal(err)
	}
	defer mpdc.Close()
	info, err := mpdc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if info == nil {
		t.Fatalf("Unexpected nil value")
	}
	if _, ok := (*info)["songid"]; !ok {
		t.Fatalf("no song id found")
	}
}

func TestCurrentSong(t *testing.T) {
	mpdc, err := Connect(mpdHost, mpdPort)
	if err != nil {
		t.Fatal(err)
	}
	defer mpdc.Close()
	info, err := mpdc.CurrentSong()
	if err != nil {
		t.Fatal(err)
	}
	if info == nil {
		t.Fatalf("Unexpected nil value")
	}
	if _, ok := (*info)["Title"]; !ok {
		t.Fatalf("no title found")
	}
}

// TestUnexistingStickerGet tests that StickerGet
// returns an empty string and no error
// when a sticker is not found
func TestUnexistingStickerGet(t *testing.T) {
	mpdc, err := Connect(mpdHost, mpdPort)
	if err != nil {
		t.Fatal(err)
	}
	defer mpdc.Close()
	value, err := mpdc.StickerGet(
		"song",
		"tests/does-not-exist.ogg",
		"playcount",
	)
	if err == nil {
		t.Fatal("Found an unexisting song")
	}
	if len(value) != 0 {
		t.Fatalf("Return string value is not empty")
	}
}

// TestExistingStickerGet tests that StickerGet
// returns a non-empty string and no error
// when a sticker is found
func TestExistingStickerGet(t *testing.T) {
	mpdc, err := Connect(mpdHost, mpdPort)
	if err != nil {
		t.Fatal(err)
	}
	defer mpdc.Close()
	existingStickerGet(t, mpdc)
	existingStickerGet(t, mpdc)
}

func existingStickerGet(t *testing.T, mpdc *MPDClient) {
	err := mpdc.StickerSet(
		"song",
		"tests/song.ogg",
		"test",
		"1",
	)
	value, err := mpdc.StickerGet(
		"song",
		"tests/song.ogg",
		"test",
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
	err = mpdc.Subscribe("whatever")
	if err != nil {
		t.Fatal(err)
	}
	err = mpdc.Unsubscribe("whatever")
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
	const channel = "whatever"
	const msg = "heya"

	done := make(chan struct{})
	go func() {
		subSub := mpdc.Idle("subscription")
		var subsystem string

		for s := 0; s < 2; s++ {
			subsystem = <-subSub.Ch
			if subsystem != "subscription" {
				t.Fatalf("Expected idle event %s, got %s", "subscription", subsystem)
			}
		}
		subSub.Close()
		close(done)
	}()
	err = mpdc.Subscribe(channel)
	if err != nil {
		t.Fatal(err)
	}
	err = mpdc.SendMessage(channel, msg)
	if err != nil {
		t.Fatal(err)
	}

	mesSub := mpdc.Idle("message")
	<-mesSub.Ch
	mesSub.Close()

	messages, err := mpdc.ReadMessages()
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("Expected %d message, got %d", 1, len(messages))
	}
	channelMessage := messages[0]
	if channelMessage.Channel != "whatever" {
		t.Fatalf("Expected channel %s, got %s", channel, channelMessage.Channel)
	}
	if channelMessage.Message != "heya" {
		t.Fatalf("Expected channel %s, got %s", channel, channelMessage.Message)
	}
	err = mpdc.Unsubscribe(channel)
	if err != nil {
		t.Fatal(err)
	}

	<-done
}

func TestIdleModeSequence(t *testing.T) {
	mpdc, err := Connect(mpdHost, mpdPort)
	if err != nil {
		t.Fatal(err)
	}
	defer mpdc.Close()

	const channelName = "test-channel"

	var idleTests = []idleTestCase{
		{"Idle 1", []string{"subscription"}, []string{"subscription"}},
		{"Idle 2", []string{"subscription", "message"}, []string{"subscription", "message"}},
		{"Idle 3", []string{"message"}, []string{"message"}},
	}

	idleTestsCompletions := make(chan idleTestCase)

	for _, idleTest := range idleTests {
		go func(idleTest idleTestCase) {
			for _, expectedSubsystem := range idleTest.ExpectedSubsystemsNotifications {
				sub := mpdc.Idle(idleTest.Subsystems...)
				subsystem := <-sub.Ch
				sub.Close()
				if subsystem != expectedSubsystem {
					t.Errorf("%s: expected subsystem %s, got %s", idleTest.Name, expectedSubsystem, subsystem)
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

	timeout := time.After(2 * time.Second)
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
				t.Fatalf("Not all idle events were received. Expected %d, got %d", len(idleTests), len(idleTests)-n)
			}

		}
	}
}

func TestConcurrentCmds(t *testing.T) {
	mpdc, err := Connect(mpdHost, mpdPort)
	if err != nil {
		t.Fatal(err)
	}
	defer mpdc.Close()

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		info, err := mpdc.Status()
		if err != nil {
			t.Fatal(err)
		}
		if info == nil {
			t.Fatalf("Unexpected nil value")
		}
		wg.Done()
	}()
	go func() {
		err = mpdc.Subscribe("whatever")
		if err != nil {
			t.Fatal(err)
		}
		err = mpdc.SendMessage("whatever", "hello MPD")
		if err != nil {
			t.Fatal(err)
		}
		wg.Done()
	}()
	go func() {
		value, err := mpdc.StickerGet(
			"song",
			"does-not-exist.mp3",
			"test",
		)
		if err == nil {
			t.Fatal("Found an unexisting song")
		}
		if len(value) != 0 {
			t.Fatalf("Return string value is not empty")
		}
		wg.Done()
	}()

	wg.Wait()
}

func TestSequence(t *testing.T) {
	mpdc, err := Connect(mpdHost, mpdPort)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		mpdc.Close()
	}()

	for i := 0; i < 5; i++ {
		info, err := mpdc.Status()
		if err != nil {
			t.Fatal(err)
		}

		if _, ok := (*info)["songid"]; !ok {
			t.Fatalf("no songid")
		}
	}
}

func TestMpdResponseFailureRegexp(t *testing.T) {
	tests := []regexpTestCase{
		regexpTestCase{
			`ACK [50@1] {play} song doesn't exist: "10240"`,
			[]string{"50", "1", "play", `song doesn't exist: "10240"`},
		},
	}
	for _, test := range tests {
		if err := test.Validate(mpdErrorRegexp); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMpdVersionRegexp(t *testing.T) {
	tests := []regexpTestCase{
		regexpTestCase{
			`OK MPD 0.12.2`,
			[]string{"0", "12", "2"},
		},
	}
	for _, test := range tests {
		if err := test.Validate(mpdVersionRegexp); err != nil {
			t.Fatal(err)
		}
	}
}
