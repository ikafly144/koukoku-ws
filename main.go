package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var ws_conn = map[*websocket.Conn]struct{}{}
var tcp_conn *tls.Conn

func init() {
	config := tls.Config{Certificates: []tls.Certificate{}, InsecureSkipVerify: false}
	conn, err := tls.Dial("tcp", "koukoku.shadan.open.ad.jp:992", &config)
	if err != nil {
		panic(err)
	}
	tcp_conn = conn
}

func main() {
	defer tcp_conn.Close()
	slog.Info("connected tls", "addr", tcp_conn.RemoteAddr())

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, r.Header)
		if err != nil {
			slog.Error("failed to upgrade request", "err", err)
			return
		}

		go wsHandler(ws)
	})

	go write()

	slog.Info("listening", "ip", "0.0.0.0", "port", "8080")
	if err := http.ListenAndServe("0.0.0.0:8080", http.DefaultServeMux); err != nil {
		panic(err)
	}

}

func wsHandler(ws *websocket.Conn) {
	defer ws.Close()
	ws_conn[ws] = struct{}{}
	var wg sync.WaitGroup
	wg.Add(1)
	go read(ws, &wg)

	wg.Wait()
	delete(ws_conn, ws)
}

func read(ws *websocket.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		_, buf, err := ws.ReadMessage()
		if err != nil {
			slog.Error("failed to read message", "err", err)
			return
		}

		slog.Info("recv from ws", "message", string(buf))

		if _, err := fmt.Fprintln(tcp_conn, string(buf)); err != nil {
			slog.Error("failed to write koukoku", "err", err)
		}
	}
}

func write() {
	scanner := bufio.NewScanner(tcp_conn)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "\x1b[0m\x1b[1m\x1b[31m>>") {
			continue
		}
		line = strings.TrimPrefix(line, "\x1b[0m\x1b[1m\x1b[31m")
		slog.Info("received", "message", line)
		for c := range ws_conn {
			if err := c.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
				slog.Error("failed to write ws", "err", err)
				continue
			}
		}
	}
}
