package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	server "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

const (
    extensionName = "nats-extension"
    eventsPath    = "/2020-01-01/extension"
)

type RegisterRequest struct {
    Events []string `json:"events"`
}

type NextEventResponse struct {
    EventType string `json:"eventType"`
    // Additional fields can be added if needed, e.g., DeadlineMs, RequestID, etc.
}

func main() {

    // touch file /tmp/nats-extension.lock so we can validate extension is up within lambda
    lockFile, err := os.Create("/tmp/nats-extension.lock")
    if err != nil {
        log.Fatalf("Failed to create lock file: %v", err)
    }
    defer lockFile.Close()

    runtimeAPI := os.Getenv("AWS_LAMBDA_RUNTIME_API")
    if runtimeAPI == "" {
        log.Fatal("AWS_LAMBDA_RUNTIME_API environment variable is not set")
    }

    // Create an embedded NATS server configured as a leaf node to PEER_NATS_URL.
    natsURL := os.Getenv("PEER_NATS_URL")
    if natsURL == "" {
        natsURL = "nats://localhost:4222"
    }
 
    // Parse the peer URL for leafnode configuration.
    peerURL, err := url.Parse(natsURL)
    if err != nil {
        log.Fatalf("Invalid PEER_NATS_URL: %v", err)
    }
 
    // Configure the embedded server to listen on localhost:4222 and connect as a leaf node.
    opts := &server.Options{
        Host: "127.0.0.1",
        Port: 4222,
        LeafNode: server.LeafNodeOpts{
            Remotes: []*server.RemoteLeafOpts{
                {
                    URLs: []*url.URL{peerURL},
                },
            },
        },
    }
 
    s, err := server.NewServer(opts)
    if err != nil {
        log.Fatalf("Failed to create embedded NATS server: %v", err)
    }
    s.ConfigureLogger()
    // Run the server in a goroutine (server.Run will block).
    go func() {
        if err := server.Run(s); err != nil {
            log.Fatalf("NATS server exited: %v", err)
        }
    }()
 
    // Wait and retry connecting a client to the embedded server.
    localURL := "nats://127.0.0.1:4222"
    var nc *nats.Conn
    for i := 0; i < 50; i++ {
        nc, err = nats.Connect(localURL)
        if err == nil {
            break
        }
        time.Sleep(100 * time.Millisecond)
    }
    if err != nil {
        // Ensure server is shutdown on failure to connect.
        if s != nil {
            s.Shutdown()
        }
        log.Fatalf("Failed to connect to embedded NATS server: %v", err)
    }
    // Close client and shutdown server when the extension exits.
    defer func() {
        nc.Close()
        if s != nil {
            s.Shutdown()
        }
    }()

    // Register the extension
    registerURL := fmt.Sprintf("http://%s%s/register", runtimeAPI, eventsPath)
    reqBody := RegisterRequest{Events: []string{"INVOKE", "SHUTDOWN"}}
    jsonBody, err := json.Marshal(reqBody)
    if err != nil {
        log.Fatalf("Failed to marshal register request: %v", err)
    }

    req, err := http.NewRequest(
        "POST", 
        registerURL, 
        bytes.NewBuffer(jsonBody),
    )
    if err != nil {
        log.Fatalf("Failed to create register request: %v", err)
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Lambda-Extension-Name", extensionName)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        log.Fatalf("Failed to register extension: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := ioutil.ReadAll(resp.Body)
        log.Fatalf("Register failed with status %d: %s", resp.StatusCode, string(body))
    }

    extensionID := resp.Header.Get("Lambda-Extension-Identifier")
    if extensionID == "" {
        log.Fatal("Missing Lambda-Extension-Identifier header")
    }

    // Publish "started" message after successful registration
    if err := nc.Publish("lambda", []byte("Extension started")); err != nil {
        log.Printf("Failed to publish start message: %v", err)
    }

    // Event loop to process INVOKE and SHUTDOWN events
    nextURL := fmt.Sprintf("http://%s%s/event/next", runtimeAPI, eventsPath)
    for {
        req, err := http.NewRequest("GET", nextURL, nil)
        if err != nil {
            log.Fatalf("Failed to create next event request: %v", err)
        }
        req.Header.Set("Lambda-Extension-Identifier", extensionID)

        resp, err := client.Do(req)
        if err != nil {
            log.Fatalf("Failed to get next event: %v", err)
        }
        body, err := ioutil.ReadAll(resp.Body)
        resp.Body.Close()
        if err != nil {
            log.Fatalf("Failed to read next event response: %v", err)
        }

        if resp.StatusCode != http.StatusOK {
            log.Fatalf("Next event failed with status %d: %s", resp.StatusCode, string(body))
        }

        var event NextEventResponse
        if err := json.Unmarshal(body, &event); err != nil {
            log.Fatalf("Failed to unmarshal event: %v", err)
        }

        switch event.EventType {
        case "INVOKE":
            // Publish "invoked" message
            if err := nc.Publish("lambda", []byte("Function invoked")); err != nil {
                log.Printf("Failed to publish invoke message: %v", err)
            }
        case "SHUTDOWN":
            // Publish "shutdown" message
            if err := nc.Publish("lambda", []byte("Extension shutting down")); err != nil {
                log.Printf("Failed to publish shutdown message: %v", err)
            }
            // Exit the extension
            return
        default:
            log.Printf("Received unknown event type: %s", event.EventType)
        }
    }
}
