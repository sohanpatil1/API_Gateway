package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	for i:=0; i<100; i++ {
		wg.Add(1)
		go func(){
			defer wg.Done()
			body := fmt.Sprintf("%d", i)
			resp,err := http.Post("http://localhost:8081/echo", "text/plain", bytes.NewBuffer([]byte(body)))
			if err != nil {
				log.Printf("Request failed to even connect: %v", err)
			}
			respBody, err := io.ReadAll((resp.Body))
			log.Printf(string(respBody))
			resp.Body.Close()	// Need to close this once done
		}()
	}
	wg.Wait()

}	
