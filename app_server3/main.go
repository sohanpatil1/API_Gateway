package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type registered_server struct {
	URL string `json:"url"`
	Port string `json:"port"`
}

func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        fmt.Printf("[%s] %s %s\n", start.Format(time.RFC3339), r.Method, r.URL.Path)

        next.ServeHTTP(w, r) // Call the actual handler
    })
}

func echoHandler(resp http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost{
		http.Error(resp, "Cannot use this method, use POST", http.StatusMethodNotAllowed)
		return
	}

	body, error := io.ReadAll(request.Body)	//body is of type bytes
	if error != nil{
		http.Error(resp, "Failed to read the body", http.StatusBadRequest)
		return
	}
	defer request.Body.Close()
	// time.Sleep(2*time.Second)	// Simulates longer processing time

	resp.Header().Set("Content-Type", "text/plain")	// The output is going to be of text type
	resp.WriteHeader(http.StatusOK)
	fmt.Fprint(resp, string(body))
}

func register_server(server_port string) bool{
	server_details := &registered_server{
		Port: server_port,
		URL: "http://localhost:"+server_port,
	}
	server_json, _ := json.Marshal(server_details)
	resp, err := http.Post("http://localhost:8080/registerServer", "application/json", bytes.NewBuffer(server_json))
	if err != nil {
		log.Fatalf("Could not register server %v",err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		fmt.Println("Server already added")
		return true
	}
	return true
}

func exit_gateway(server_port string) {
	log.Println("Control-c detected turning off system.")
	resp, err := http.Post("http://localhost:8080/exit", "text/plain", bytes.NewBuffer([]byte(server_port)))
	if err != nil {
		log.Fatal("Could not exit out of gateway cleanly.")
	}
	defer resp.Body.Close()
	log.Println("Shutting down cleanly")
}

func healthCheck (w http.ResponseWriter, req *http.Request){
	/*
	Simple healthCheck function call
	*/
	w.WriteHeader(http.StatusOK)
}

func main() {
	signal_shutdown := make(chan os.Signal, 1)
	signal.Notify(signal_shutdown, os.Interrupt, syscall.SIGTERM)	// Setting all triggers for a shutdown.

	server_port := "8083"
	if !register_server(server_port){
		log.Printf("[WARNING] Could not register server.")
		os.Exit(1) 
	}
	
	// System exit call
	go func() {
		<-signal_shutdown // No need for LHS because we dont need to store the channel data anywhere
		// Perform exit strategy
		exit_gateway(server_port)
		os.Exit(0)
	}()


	// Opentelemetry start and shutdown
	tp, err := initTracer("app_server3")
	if err != nil {
		log.Fatalf("Could not start tracer: %v", err)
	}
	
	defer shutdownTracer(tp, context.Background())

	log.Printf("Registered the server")
	mux := http.NewServeMux()
    mux.Handle("/echo", 
		otelhttp.NewHandler(
			http.HandlerFunc(echoHandler),
			"echo-app_server-handler",
		),
	)	// Function that runs when endpoint is reached

	mux.Handle("/health", 
		otelhttp.NewHandler(
			http.HandlerFunc(healthCheck),
			"app_server-healthCheck",
		),
	)	// Function that runs when endpoint is reached
	loggedMux := loggingMiddleware(mux)
	
	log.Printf("Server starting on port %s", server_port)
	addr := fmt.Sprintf(":%s", server_port)
	err = http.ListenAndServe(addr, loggedMux)	// blocks and runs indefinitely
	if err != nil {
		fmt.Println("There was an error starting the server", err)
	}
}