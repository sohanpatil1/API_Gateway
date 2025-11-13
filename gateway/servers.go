package main

import (
	"container/heap"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type server_struct struct {
	URL string
	port int
	alive bool
	last_updated int64
	mu sync.RWMutex
	in_queue int
	UID string
	index int
}

var servers = make(map[string]*server_struct)	//string UID and value is servers struct

// Heap for queue
type ServerHeap []*server_struct // Get the server with the lowest load (queue)
func (sh ServerHeap) Len() int           { return len(sh) }
func (sh ServerHeap) Less(i, j int) bool { return sh[i].in_queue < sh[j].in_queue}
func (sh ServerHeap) Swap(i, j int) {
    sh[i], sh[j] = sh[j], sh[i]
	sh[i].index = i
	sh[j].index = j
}
func (sh *ServerHeap) Push(element any){
	n := len(*sh)
	element.(*server_struct).index = n
	*sh = append(*sh, element.(*server_struct))
}
func (sh *ServerHeap) Pop() any {
	old_heap := *sh
	old_len := len(old_heap)
	popped_element := old_heap[old_len-1]
	*sh = old_heap[0 : old_len-1]
	return popped_element
}

var sh ServerHeap //Actual global variable

func isAlive(url string, port int) bool{
	resp, err := http.Get("http://" + url + ":" + strconv.Itoa(port) + "/health")
	if err!=nil{
		return false
	}
	defer resp.Body.Close()
	return true
}

func get_next_server() *server_struct{
	if sh.Len() == 0{
		return nil
	}
	server := heap.Pop(&sh).(*server_struct)
	return server
}

func remove_server(server *server_struct) bool {
	// Remove from heap
	found := false
	for index, s := range(sh) {
		if s.UID == server.UID {
			heap.Remove(&sh, index)
			found = true
			break
		}
	}
	// Remove from map
	if _, ok := servers[server.UID]; ok {
		delete(servers, server.UID)
		found = true
	}
	if found {
		log.Printf("Deleted server %s cleanly", server.UID)
		return true
	}

	return false // server not found in heap or map
}

func start_heartbeat() {
	for {
		for UID, server := range servers {
			fmt.Println("Found server " + UID)
			server.mu.Lock()
			if isAlive(UID, server.port){
				server.alive = true
			} else{
				// TODO set it on the mock_api side too
				server.alive = false
			}
			server.last_updated = time.Now().Unix()
			server.mu.Unlock()
		}
		log.Println("Heartbeat check done")
		time.Sleep(5*time.Second)
	}
}