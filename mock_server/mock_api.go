package main

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

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
	time.Sleep(2*time.Second)	// Simulates longer processing time

	resp.Header().Set("Content-Type", "text/plain")	// The output is going to be of text type
	resp.WriteHeader(http.StatusOK)
	fmt.Fprint(resp, string(body))

}

func main() {
	// http.HandleFunc("/echo", echoHandler)	// Function that runs when endpoint is reached
	mux := http.NewServeMux()
    mux.HandleFunc("/echo", echoHandler)
	loggedMux := loggingMiddleware(mux)
	err := http.ListenAndServe(":8080", loggedMux)	// blocks and runs indefinitely
	if err != nil {
		fmt.Println("There was an error starting the server", err)
	} else{
		fmt.Println("Server running successfully on port 8080")
	}
}