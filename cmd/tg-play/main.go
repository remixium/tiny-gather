// Command tg-play is the human client: it joins a server and renders the game
// in the terminal.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/vadremix/tiny-gather/client/conn"
	"github.com/vadremix/tiny-gather/client/render/term"
)

func main() {
	if err := run(); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "tg-play: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		url   = flag.String("url", "ws://localhost:8080/play", "server to join")
		name  = flag.String("name", "", "name to join as (new player)")
		token = flag.String("token", "", "reconnect to an existing player with this token")
	)
	flag.Parse()

	if *name == "" && *token == "" {
		return errors.New("pass -name to join, or -token to reconnect")
	}

	dial, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	c, err := conn.Dial(dial, *url, conn.Options{Name: *name, Token: *token})
	cancel()
	if err != nil {
		return err
	}
	defer c.Close()

	err = term.Run(context.Background(), c)

	// The token is what gets this player back after quitting. It is printed
	// on the way out rather than stored anywhere, so it is the user's choice
	// where it goes.
	fmt.Fprintf(os.Stderr, "reconnect with: -token %s\n", c.Token())
	return err
}
