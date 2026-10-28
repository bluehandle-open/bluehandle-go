// Command bluehandle connects to the phone and replays its key events.
//
// Phase 1 ships a command-line program that prints keystrokes instead of
// injecting them; the robotgo keyboard and the Fyne window come later.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/bluehandle-open/bluehandle-go/internal/client"
	"github.com/bluehandle-open/bluehandle-go/internal/input"
	"github.com/bluehandle-open/bluehandle-go/internal/keymap"
)

type logEvents struct{}

func (logEvents) OnState(s client.State, detail string) { log.Printf("state: %s %s", s, detail) }
func (logEvents) OnError(err error)                     { log.Printf("error: %v", err) }

func main() {
	ip := flag.String("ip", "", "phone IP address (required)")
	keys := flag.String("keys", "", "key table to push after connecting: handle, ppt or player")
	flag.Parse()
	if *ip == "" {
		fmt.Fprintln(os.Stderr, "usage: bluehandle -ip <phone ip> [-keys handle|ppt|player]")
		os.Exit(2)
	}
	var keySet byte
	if *keys != "" {
		typ, ok := keymap.TypeByName(*keys)
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown key table %q\n", *keys)
			os.Exit(2)
		}
		keySet = typ
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	c := client.New(input.Printer{W: os.Stdout}, logEvents{})
	if err := c.Connect(ctx, *ip); err != nil {
		log.Fatal(err)
	}
	if keySet != 0 {
		if err := c.SendKeySetting(keySet); err != nil {
			log.Printf("push key table: %v", err)
		}
	}

	select {
	case <-ctx.Done():
		if err := c.Close(); err != nil {
			log.Printf("close: %v", err)
		}
		<-c.Done()
	case <-c.Done():
	}
}
