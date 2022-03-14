package main

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/ckin-it/minedive/minedive"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func Test_mined(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)

		setupTest(t)
		cl, err := newClient(ctx, "ws://localhost:6999/")
		assertSuccess(t, err)
		cancel()
		defer cl.c.Close(websocket.StatusGoingAway, "")

		ctx, cancel = context.WithTimeout(context.Background(), time.Second*10)
		msg := minedive.Cell{Type: "ping", D0: "AAA"}
		err = wsjson.Write(ctx, cl.c, msg)
		assertSuccess(t, err)
		cancel()
		ctx, cancel = context.WithTimeout(context.Background(), time.Second*10)
		err = wsjson.Read(ctx, cl.c, &msg)
		assertSuccess(t, err)
		cancel()
		log.Println(msg)

		ctx, cancel = context.WithTimeout(context.Background(), time.Second*10)
		msg = minedive.Cell{Type: "gettssid", D0: ""}
		err = wsjson.Write(ctx, cl.c, msg)
		assertSuccess(t, err)
		cancel()
		ctx, cancel = context.WithTimeout(context.Background(), time.Second*10)
		err = wsjson.Read(ctx, cl.c, &msg)
		assertSuccess(t, err)
		cancel()
		log.Println(msg)

		ctx, cancel = context.WithTimeout(context.Background(), time.Second*10)
		msg = minedive.Cell{Type: "gettssid", D0: ""}
		err = wsjson.Write(ctx, cl.c, msg)
		assertSuccess(t, err)
		cancel()
		ctx, cancel = context.WithTimeout(context.Background(), time.Second*10)
		err = wsjson.Read(ctx, cl.c, &msg)
		assertSuccess(t, err)
		cancel()
		log.Println(msg)
	})
}

type client struct {
	url string
	c   *websocket.Conn
}

func newClient(ctx context.Context, url string) (*client, error) {
	ccc := websocket.DialOptions{}
	ccc.Subprotocols = append(ccc.Subprotocols, "json")
	c, _, err := websocket.Dial(ctx, url, &ccc)
	if err != nil {
		return nil, err
	}

	cl := &client{
		url: url,
		c:   c,
	}

	return cl, nil
}

func setupTest(t *testing.T) {
	go runMined("", true, 6999)
}

func assertSuccess(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
