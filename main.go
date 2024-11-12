package main

import (
    "fmt"
    "net/http"
    "io/ioutil"
    "encoding/json"
    "crypto/ed25519"
	"encoding/hex"
    "log"
    "strings"
	"path/filepath"
    "bytes"
    "io"

    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3"
)

func createS3Client() *s3.S3 {
    sess, err := session.NewSession(&aws.Config{
        Region: aws.String("ap-southeast-1"),
    })
    if err != nil {
        log.Fatalf("Failed to create session: %v", err)
    }
    return s3.New(sess)
}

func uploadFile(svc *s3.S3, bucket string, file *bytes.Reader, key string) error {
    _, err := svc.PutObject(&s3.PutObjectInput{
        Bucket: aws.String(bucket),
        Key:    aws.String(key),
        Body:   file,
    })
    if err != nil {
        return fmt.Errorf("failed to upload file, %v", err)
    }

    fmt.Printf("File uploaded successfully to s3://%s/%s\n", bucket, key)
    return nil
}

func downloadFile(svc *s3.S3, bucket string, key string) error {
    res, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
    return err
}

type Request struct {
    Type int `json:"type"`
    Data struct {
        Name string `json:"name"`

        Options []struct {
            Value string `json:"value"`
        } `json:"options"`

        Resolved struct {
            Attachments map[string]Attachment `json:"attachments"`
        } `json:"resolved"`
    } `json:"data"`
}

type Attachment struct {
    Filename string `json:"filename"`
    Url string `json:"url"`
}

type Response struct {
    Type int `json:"type"`
    Data struct {
        Content string `json:"content"`
    } `json:"data"`
}

const PUBLIC_KEY = "6d60e10fd7a4c29556fbb804df4e943dade4f634ee2f27e79d92a477bce94690"

func verifySignature(signatureHex, timestamp, body string) bool {
    publicKey, err := hex.DecodeString(PUBLIC_KEY)
    if err != nil {
        return false
    }
    signature, err := hex.DecodeString(signatureHex)
    if err != nil {
        return false
    } 
    message := []byte(timestamp + body)
    return ed25519.Verify(publicKey, message, signature)
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

    // Verify security headers
    signature := r.Header.Get("X-Signature-Ed25519")
    timestamp := r.Header.Get("X-Signature-Timestamp")
    if !verifySignature(signature, timestamp, string(body)) {
        http.Error(w, "Invalid request signature", http.StatusUnauthorized)
        return
    }
    
    var rData Request
    err = json.Unmarshal(body, &rData)
    if err != nil {
        http.Error(w, "400 - Bad Request", http.StatusBadRequest)
        return
    }

    // Ping
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

    if rData.Type != 2 {
        http.Error(w, "Unsupported interaction", http.StatusBadRequest)
        return
    }

    // Create s3 client
    svc := createS3Client()

    // Json response
    w.Header().Set("Content-Type", "application/json")
    if rData.Data.Name == "get" {
        res := Response{
            Type: 4,
        }

        files := strings.Split(rData.Data.Options[0].Value, " ")
        for _, file := range files {
            file = filepath.Clean(file)

            res.Data.Content += "* Successfully dowloaded " + file + "\n"
        }

        json.NewEncoder(w).Encode(res) 
        return
    }

    if rData.Data.Name == "upload" {
        for _, attachment := range rData.Data.Resolved.Attachments {
            res, err := http.Get(attachment.Url)
            if err != nil {
                http.Error(w, "Error uploading file", http.StatusInternalServerError)
                return
            }

            var buf bytes.Buffer
	        _, err = io.Copy(&buf, res.Body)
	        if err != nil {
                http.Error(w, "Error uploading file", http.StatusInternalServerError)
		        return
	        }
            defer res.Body.Close()

            err = uploadFile(svc, "nyi", bytes.NewReader(buf.Bytes()), attachment.Filename)
            if err != nil {
                http.Error(w, "Error uploading file", http.StatusInternalServerError)
                return
            }
        }

        res := Response{
            Type: 4,
        }

        res.Data.Content = "Successfully uploaded files!"
        json.NewEncoder(w).Encode(res) 
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