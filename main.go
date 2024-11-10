package main

import (
    "fmt"
    "net/http"
    "io/ioutil"
    "encoding/json"
)

type Request struct {
    Type int `json:"type"`
    Data struct {
        Name string `json:"name"`

        Options []struct {
            Value string `json:"value"`
        } `json:"options"`

    } `json:"data"`
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "405 - Method Not Allowed", http.StatusMethodNotAllowed)
        return
    }

    body, err := ioutil.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "400 - Bad Request", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()
    
    var rData Request
    err = json.Unmarshal(body, &rData)
    if err != nil {
        http.Error(w, "400 - Bad Request", http.StatusBadRequest)
        return
    }

    if rData.Type == 1 {
        response := map[string]int{
            "type": 1,
        }
        w.Header().Set("Content-Type", "application/json")

        if err := json.NewEncoder(w).Encode(response); err != nil {
            http.Error(w, "Unable to encode JSON", http.StatusInternalServerError)
        }
        return
    }
}

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", rootHandler)

    fmt.Println("Starting server on port 2010...")
    err := http.ListenAndServe(":2010", mux)
    if err != nil {
        panic(err)
    }
}