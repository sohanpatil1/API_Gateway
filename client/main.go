// mock_client

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
			resp,err := http.Post("http://localhost:8080/echo", "text/plain", bytes.NewBuffer([]byte(body)))
			if err != nil {
				log.Printf("Cannot connect to 8080: %v",err)
				return
			}
			respBody, _ := io.ReadAll((resp.Body))
			log.Printf("%s", respBody)
			resp.Body.Close()	// Need to close this once done
		}()
	}
	wg.Wait()
}	
