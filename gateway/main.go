package main
import (
	"fmt"
	"io"
	"net/http"
	"bytes"
	"log"
	"container/list"
	"sync"
	"encoding/json"
	"time"
	"net"
	"container/heap"
)

var rate_limiting_cache = make(map[string]*list.List)
var mutex sync.Mutex

type registered_server struct {
	URL string `json:"url"`
	Port string `json:"port"`
}

func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        log.Printf("[%s] %s %s\n", start.Format("15:04"), r.Method, r.URL.Path)

        next.ServeHTTP(w, r) // Call the actual handler

		log.Printf("[%s] %s %s Done\n", start.Format("15:04"), r.Method, r.URL.Path)
    })
}

func rate_limiter(request *http.Request) bool {
	client_ip, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		// RemoteAddr might already be just an IP (rare), so fall back:
		client_ip = request.RemoteAddr
	}
	var client_time = time.Now()
	mutex.Lock()
	defer mutex.Unlock()
	value, exists := rate_limiting_cache[client_ip]
	if !exists {
		new_list := list.New()
		rate_limiting_cache[client_ip] = new_list
		new_list.PushBack(client_time)
	} else{
		// Clean up old queries beyond time window
		current_time := time.Now()
		for {
			oldest := value.Front()
			if oldest == nil{	// list is empty
				break
			}
			oldest_time := oldest.Value.(time.Time) 
			//its type is interface{}, so Go doesn’t know it’s a time.Time — you must assert that manually
			difference := current_time.Sub(oldest_time).Seconds()
			if difference > 0.5 {
				value.Remove(oldest)
			} else{
				break
			}
		}
		value.PushBack(client_time)
		if value.Len() >= 35{
			return false
		}
	}
	return true
}


func echoHandler(initial_response http.ResponseWriter, initial_request *http.Request) {
	safetogo := rate_limiter(initial_request)
	if !safetogo{
		http.Error(initial_response, "Too many requests", http.StatusTooManyRequests)
		return
	}

	body, err := io.ReadAll(initial_request.Body)	//body is of type bytes
	if err != nil{
		http.Error(initial_response, "Failed to read the body", http.StatusBadRequest)
		return
	}
	defer initial_request.Body.Close()

	// Calls a function that returns which endpoints are available to use. 
	// BTS it keeps checking and updating the available endpoints
	if len(sh) == 0 {
		http.Error(initial_response, "No upstream servers available", http.StatusServiceUnavailable)
		return
	}
	server := sh[0]
	url := server.URL+"/echo"
	server.mu.Lock()
	server.in_queue ++
	heap.Fix(&sh, server.index)
	server.mu.Unlock()
	// Post is a blocking call, make it async
	log.Printf("Gateway making a Post call to %s", url)
	response, err := http.Post(url, "text/plain", bytes.NewBuffer(body))	// bytes not allowed, need io.Reader
	if err != nil{
		http.Error(initial_response, err.Error(), http.StatusBadGateway)
		return
	} 
	defer response.Body.Close()
	log.Println("Response status: ", response.Status)
	
	if response.StatusCode != http.StatusOK {
		http.Error(initial_response, "Upstream server error", http.StatusBadGateway)
		return
	}
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		http.Error(initial_response, "Failed to read body from upstream", http.StatusBadGateway)
		return
	}

	// Send API response back to the client from mock server
	initial_response.Header().Set("Content-Type", "text/plain")	// The output is going to be of text type
	initial_response.WriteHeader(http.StatusOK)
	fmt.Fprint(initial_response, string(responseBody))
	server.mu.Lock()
	server.in_queue -= 1
	server.mu.Unlock()
}

func registerServer(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost{
		http.Error(w, "Did not use the right method. Use Get", http.StatusBadRequest)
		return
	}
	var server registered_server
    err := json.NewDecoder(req.Body).Decode(&server)
	if err != nil {
		http.Error(w, "Could not read server JSON", http.StatusBadRequest)
	}
	var url string
	var port string

	url = server.URL
	port = server.Port
	if err != nil{
		log.Println("Errored out during reading of port.")
		return
	}

	_,exists := servers[port]
	if exists{
		http.Error(w, "Server already added", http.StatusConflict)
		return
	}

	servers[port] = &server_struct{
		URL: url,
		alive: true,
		last_updated: time.Now().Unix(),
		port: port,
	}
	heap.Push(&sh, servers[port])	// Add to heap as well

	log.Printf("Server URL : %s port: %s connected successfully", url, port)
	w.WriteHeader(http.StatusOK)	// Sends the status code back to client
	fmt.Fprintf(w, "Server %s connected successfully", req.RemoteAddr)
}

func exitServer(w http.ResponseWriter, req *http.Request){
	if req.Method != http.MethodPost {
		http.Error(w, "Need to use POST call for /exit", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(req.Body)	//body is of type bytes
	if err != nil{
		http.Error(w, "Failed to read the Port", http.StatusBadRequest)
		return
	}

	server, exists := servers[string(body)]	// TODO Fails here. Searching based on port but key is UID
	if !exists || !remove_server(server) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Server %s could not be found", string(body))
		log.Printf("Server %s could not be found", string(body))
		return
	}
	log.Printf("Removed server with port %s", string(body))
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Removed server with port %s", string(body))
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("This is the gateway module")
	
	mux := http.NewServeMux()
    mux.HandleFunc("/echo", echoHandler)	// Function that runs when endpoint is reached
	mux.HandleFunc("/registerServer", registerServer)
	mux.HandleFunc("/exit", exitServer)

	heap.Init(&sh)
	
	loggedMux := loggingMiddleware(mux)
	go start_heartbeat()	// Start heartbeat service in the background
	log.Println("Server starting on port 8080")
	err := http.ListenAndServe(":8080", loggedMux)	// blocks and runs indefinitely
	if err != nil {
		log.Println("There was an error starting the server", err)
	}
}