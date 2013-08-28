package mpdfav

import (
	"errors"
	"fmt"
	"log"
	"strconv"
)

const (
	ratingSticker = "rating"
	RatingsChannel = "ratings"
)

func rateSong(songInfo *Info, rateMsg string, mpdc *MPDClient) (int, error) {
	// fail fast if the rateMsg is invalid
	var val int
	switch rateMsg {
	case "+":
		fallthrough
	case "like":
		val = 1
	case "-":
		fallthrough
	case "dislike":
		val = -1
	default:
		val = 0
	}
	if val == 0 {
		return -1, errors.New(fmt.Sprintf("Invalid rating code: %s", rateMsg))
	}

	value, err := mpdc.StickerGet(
		StickerSongType,
		(*songInfo)["file"],
		ratingSticker,
	)
	if err != nil {
		return -1, err
	}
	if len(value) == 0 {
		value = "0"
	}
	intval, err := strconv.Atoi(value)
	if err != nil {
		return -1, err
	}

	intval += val

	err = mpdc.StickerSet(
		StickerSongType,
		(*songInfo)["file"],
		ratingSticker,
		strconv.Itoa(intval),
	)
	return intval, err
}

func ListenRatings(mpdc *MPDClient) {
	err := mpdc.Subscribe(RatingsChannel)
	if err != nil {
		panic(err)
	}

	statusInfo, err := mpdc.Status()
	if err != nil {
		panic(err)
	}
	currentSongId := statusInfo["songid"]

	clientsSentRating := make([]string, 0)

	msgsCh := make(chan ChannelMessage)
	playerCh := make(chan Info)

	go func() {
		for {
			subsystem := <-mpdc.Idle("message", "player")
			if err != nil {
				log.Println(err)
			} else {
				switch subsystem {
				case "message":
					msgs, err := mpdc.ReadMessages()
					if err != nil {
						log.Println(err)
					}
					for _, msg := range msgs {
						msgsCh <- msg
					}
				case "player":
					statusInfo, err := mpdc.Status()
					if err != nil {
						log.Println(err)
					}
					playerCh <- statusInfo
				}
			}
		}
	}()

	for {
		select {
		case channelMessage := <-msgsCh:
			log.Println("Ratings: incoming message", channelMessage)
			if channelMessage.Channel != RatingsChannel {
				break
			}

			// FIXME find a way to Uidentify a client submitting a rating
			thisClientId := "0"
			clientExists := false
			for _, clientId := range clientsSentRating {
				if thisClientId == clientId {
					clientExists = true
					break
				}
			}
			if !clientExists {
				if err == nil {
					songInfo, err := mpdc.CurrentSong()
					if err == nil {
						if rating, err := rateSong(&songInfo, channelMessage.Message, mpdc); err != nil {
							log.Println(err)
						} else {
							clientsSentRating = append(clientsSentRating, thisClientId)
							log.Println(songInfo["Title"], " rating=", rating)
						}
					} else {
						log.Println(err)
					}
				} else {
					log.Println(err)
				}
				mpdc.Close()
			}
		case statusInfo := <-playerCh:
			if currentSongId != statusInfo["songid"] {
				log.Println("Ratings: song changed to", statusInfo["songid"])
				clientsSentRating = make([]string, 0)
			}
		}
	}
}
