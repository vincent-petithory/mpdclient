# MPD Client for Go


# Purpose

Only a subset of MPD commands is implemented: the current implementation focused
on getting a robust idle events system, fetching mpd status, working with stickers and client-to-client messaging.

More commands could be added in the future, if a need arise,
since the current implementation allows to add new commands fairly easily.
In particular, commands not yet implemented can be run anyway using the generic Cmd() method.

However, commands lists are not implemented and the current implementation isn't suited for them.

# Example usage

## Basic commands

Import:

    import "github.com/vincent-petithory/mpdclient"

Connect:

    mpdc, err := mpdclient.Connect("localhost", 6600)
    if err != nil {
        panic(err)
    }
    // Close client when done
    defer mpdc.Close()

Print title of current song:

    info, err := mpdc.CurrentSong()
    if err != nil {
        panic(err)
    }
    fmt.Printf("Title: %s\n", (*info)["Title"])

Most commands that return data (such as `currentsong`) will return `(*Info, error)`, and those who do not will simply return `error`.

## Idle

Listening to idle events is done through an IdleListener type.
It provides a channel in which idle events will be fed.

Listen forever to all `player` and `mixer` events (player pause/stop/start/seek and volume changes):

    idleEvents := mpdc.Idle("player", "mixer")
    for {
        subsystem := <-idleEvents.Ch
        switch subsystem {
        case "player":
            fmt.Println("player status changed.")
        case "mixer":
            statusInfo, err := mpdc.Status()
            if err != nil {
                panic(err)
            } else {
                fmt.Printf("volume is: %s\n", (*statusInfo)["volume"])
            }
        }
    }

## Messaging

Subscribe to a channel and write all incoming messages to stdout:

	err := mpdc.Subscribe("mychannel")
	if err != nil {
		panic(err)
	}

	mesEvents := mpdc.Idle("message")
	for {
        <-mesEvents.Ch
        channelMessages, err := mpdc.ReadMessages()
        if err != nil {
            panic(err)
        }
        for _, channelMessage := range channelMessages {
            fmt.Printf("Got message \"%s\" on channel \"%s\"\n", channelMessage.Message, channelMessage.Channel)
        }
	}

Run:
    $ mpc sendmessage mychannel 'Did you get the message?'

## More ?

* The [unit tests](client_test.go) are also a good example.
* Checkout [mpdfav](https://github.com/vincent-petithory/mpdfav) for "real world" usage.

# Internals

One client instance opens 3 connections.
The reason is mostly to not lose any idle event when mpd commands are sent blazing fast.
Tunnelling everything in one connection was also creating races.
A later implementation may improve that and lower the connections opened.

* One connection specifically for client-to-client (subscription) idle message subsystem
* One connection for all idle subsystems
* One connection for all standard commands.
  That connection will forward the 3 following commands to the subscription idle loop: readmessages, subscribe, unsubscribe.

    subscription idle	main					idle
    |					| send noidle			|
    | wait request		|						|
    |					| send request object	|
    | process request	|						|
    | send response		|						|

