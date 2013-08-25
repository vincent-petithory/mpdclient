package mpdfav

import (
	_ "os"
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

