package main

import (
	"log"
	"net/http"
	"math/rand"
	"encoding/json"
)

var shortenedURLs = make(map[string]string);


func randomString(length int) string {
    //rand.Seed(time.Now().UnixNano())
    const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    result := make([]byte, length)
    for i := range result {
        result[i] = charset[rand.Intn(len(charset))]
    }
    return string(result)
}

func shortenURLHandler(w http.ResponseWriter, r *http.Request){
	var body struct {
		URL string `json:"url"`
	}

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.URL == ""{
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return 
	}
	
	//long_url := body.URL;//r.PathValue("url");
	rand_url := randomString(6)
	for {
    	_, ok := shortenedURLs[rand_url]
    	if !ok {
        	break
    	}
    	rand_url = randomString(6) // collision, try again
	}
	shortenedURLs[rand_url] = body.URL
	w.Write([]byte("https://short.ly/" + rand_url + "\n"))
	
}

func redirectHandler(w http.ResponseWriter, r *http.Request){
	code := r.PathValue("code");
	_ , ok := shortenedURLs[code];
	if ok == false {
		http.Error(w, "Code not found", http.StatusNotFound);
		return;
	} 
		//w.Write([]byte("The long url is: " + shortenedURLs[code]));
	http.Redirect(w,r, shortenedURLs[code], http.StatusFound);
	return;
	
}

func main(){
	
	router := http.NewServeMux();

	router.HandleFunc("POST /shorten", shortenURLHandler);
	router.HandleFunc("GET /{code}", redirectHandler);


	server := http.Server{Addr: ":8080", Handler: router};
	log.Println("Starting server on port :8080")
	log.Fatal(server.ListenAndServe());
}


