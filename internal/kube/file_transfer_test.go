package kube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/filetransfer"
	"github.com/gorilla/websocket"
)

func TestFileExecPreservesBinaryAndUsesIndependentNonTTYStream(t *testing.T) {
	payload := []byte{0, 1, 13, 10, 255, 128, 42}
	command := []string{"/bin/sh", "-c", "fixed script", "file", "/tmp/a ; touch nope"}
	for _, upload := range []bool{true, false} {
		t.Run(fmt.Sprint(upload), func(t *testing.T) {
			client := protocolClient(t, func(w http.ResponseWriter, r *http.Request) {
				query := r.URL.Query()
				if query.Get("tty") == "true" || query.Get("stderr") != "true" || query.Get("container") != "app" || !reflect.DeepEqual(query["command"], command) {
					t.Errorf("unsafe exec options: %v", query)
				}
				conn, err := (&websocket.Upgrader{Subprotocols: []string{"v5.channel.k8s.io"}}).Upgrade(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.Close()
				conn.SetReadDeadline(time.Now().Add(3 * time.Second))
				if upload {
					var received []byte
					for len(received) < len(payload) {
						_, data, err := conn.ReadMessage()
						if err != nil {
							t.Error(err)
							return
						}
						if len(data) > 0 && data[0] == 0 {
							received = append(received, data[1:]...)
						}
					}
					if !bytes.Equal(received, payload) {
						t.Error("stdin bytes changed")
					}
				}
				conn.WriteMessage(websocket.BinaryMessage, append([]byte{1}, payload...))
				conn.WriteMessage(websocket.BinaryMessage, append([]byte{2}, []byte("stderr must not enter file")...))
				conn.WriteMessage(websocket.BinaryMessage, append([]byte{3}, []byte(`{"status":"Success"}`)...))
				finishFileSocket(conn)
			})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			var input io.Reader
			if upload {
				input = bytes.NewReader(payload)
			}
			var out bytes.Buffer
			if err := client.FileExec(ctx, podRequest("exec"), command, input, &out); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out.Bytes(), payload) {
				t.Fatalf("binary stdout changed: %v", out.Bytes())
			}
		})
	}
}

func TestFileExecMapsOverwriteConflictAndRejectsReplacement(t *testing.T) {
	client := protocolClient(t, func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{Subprotocols: []string{"v5.channel.k8s.io"}}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		conn.WriteMessage(websocket.BinaryMessage, append([]byte{3}, []byte(`{"status":"Failure","reason":"NonZeroExitCode","details":{"causes":[{"reason":"ExitCode","message":"17"}]}}`)...))
		finishFileSocket(conn)
	})
	if err := client.FileExec(t.Context(), podRequest("exec"), []string{"test"}, nil, io.Discard); !errors.Is(err, filetransfer.ErrExists) {
		t.Fatalf("%v", err)
	}
	r := podRequest("exec")
	r.UID = "old"
	if err := client.FileExec(t.Context(), r, []string{"test"}, nil, io.Discard); err == nil || errors.Is(err, filetransfer.ErrExists) {
		t.Fatal("replacement reached transfer endpoint", err)
	}
}

func finishFileSocket(conn *websocket.Conn) {
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
