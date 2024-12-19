package main

import (
    "context"
    "fmt"
    "net/http"
    "net/url"
    "encoding/json"
    "crypto/ed25519"
    "encoding/hex"
    "mime/multipart"
    "log"
    "strings"
    "bytes"
    "io"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-lambda-go/events"
)

func uploadFile(bucket string, file *bytes.Reader, key string) error {
    _, err := svc.PutObject(context.TODO(), &s3.PutObjectInput{
        Bucket: aws.String(bucket),
        Key:    aws.String(key),
        Body:   file,
    })
    if err != nil {
        return err
    }

    fmt.Printf("File uploaded successfully to s3://%s/%s\n", bucket, key)
    return nil
}

func downloadFile(bucket string, key string) (*bytes.Buffer, string, error) {
    res, err := svc.GetObject(context.TODO(), &s3.GetObjectInput{
        Bucket: aws.String(bucket),
        Key:    aws.String(key),
    })
    if err != nil {
        return nil, "", err
    }
    defer res.Body.Close()

    buffer := new(bytes.Buffer)
    _, err = io.Copy(buffer, res.Body)
    if err != nil {
        return nil, "", err
    }

    fileName := key[strings.LastIndex(key, "/")+1:]
    return buffer, fileName, nil
}

type Request struct {
    Type int `json:"type"`
    ID string `json:"id"`
    Token string `json:"token"`
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

var svc *s3.Client

const PUBLIC_KEY = "6d60e10fd7a4c29556fbb804df4e943dade4f634ee2f27e79d92a477bce94690"
const APP_ID = "1305095284774404116"

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

func acknowledge(url string) {
    payload := map[string]interface{}{"type": 5}
    body, _ := json.Marshal(payload)
    http.Post(url, "application/json", bytes.NewBuffer(body))
}

func sendFileToDiscord(url string, fileBuffer *bytes.Buffer, fileName string) {
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)

    part, err := writer.CreateFormFile("file", fileName)
    if err != nil {
        fmt.Println("Error creating form file:", err)
        return
    }

    _, err = io.Copy(part, fileBuffer)
    if err != nil {
        fmt.Println("Error writing file buffer:", err)
        return
    }
    err = writer.WriteField("content", "Here is your file")
    if err != nil {
        fmt.Println("Error writing field:", err)
        return
    }

    err = writer.Close()
    if err != nil {
        fmt.Println("Error closing writer:", err)
        return
    }

    req, err := http.NewRequest("POST", url, body)
    if err != nil {
        fmt.Println("Error creating request:", err)
        return
    }

    req.Header.Set("Content-Type", writer.FormDataContentType())

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        fmt.Println("Error sending request:", err)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode == 200 {
        fmt.Println("File sent successfully!")
    } else {
        fmt.Printf("Failed to send file, status code: %d\n", resp.StatusCode)
    }
}

func rootHandler(r events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
    body := r.Body
    signature := r.Headers["x-signature-ed25519"]
    timestamp := r.Headers["x-signature-timestamp"]

    if !verifySignature(signature, timestamp, body) {
        return &events.APIGatewayProxyResponse{
            StatusCode: 401,
            Body:       "Invalid request signature",
        }, nil
    }

    var rData Request
    err := json.Unmarshal([]byte(body), &rData)
    if err != nil {
        return &events.APIGatewayProxyResponse{
            StatusCode: 400,
            Body:       "400 - Bad Request",
        }, nil
    }

    if rData.Type == 1 {
        response := map[string]int{
            "type": 1,
        }

        responseBody, err := json.Marshal(response)
        if err != nil {
            return &events.APIGatewayProxyResponse{
                StatusCode: 500,
                Body:       "Unable to encode JSON",
            }, nil
        }
        return &events.APIGatewayProxyResponse{
            StatusCode: 200,
            Body:       string(responseBody),
            Headers: map[string]string{
                "Content-Type": "application/json",
            },
        }, nil
    }

    if rData.Type != 2 {
        return &events.APIGatewayProxyResponse{
            StatusCode: 400,
            Body:       "Unsupported interaction",
        }, nil
    }

    if rData.Data.Name == "ls" {
        res := Response{
            Type: 4,
        }

        resp, err := svc.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		    Bucket: aws.String("nyi"),
	    })
        if err != nil {
            log.Fatalf("Unable to list items in bucket nyi, %v", err)
        }

        res.Data.Content = "The files are included are:\n"
        for _, item := range resp.Contents {
            res.Data.Content += "* ```" + *item.Key + "```\n"
        }
        responseBody, err := json.Marshal(res)
        if err != nil {
            return &events.APIGatewayProxyResponse{
                StatusCode: 500,
                Body:       "Unable to encode response JSON",
            }, nil
        }
        return &events.APIGatewayProxyResponse{
            StatusCode: 200,
            Body:       string(responseBody),
            Headers: map[string]string{
                "Content-Type": "application/json",
            },
        }, nil
    }

    if rData.Data.Name == "get" {
        responseURL := fmt.Sprintf("https://discord.com/api/v10/interactions/%s/%s/callback", rData.ID, rData.Token)
        acknowledge(responseURL)

        files := strings.Split(rData.Data.Options[0].Value, " ")
        followUpURL := fmt.Sprintf("https://discord.com/api/v10/webhooks/%s/%s", APP_ID, rData.Token)

        for _, file := range files {
            fileBuffer, fileName, err := downloadFile("nyi", file)
            if err != nil {
                log.Printf("Error fetching file from S3: %v", err)

	            body := &bytes.Buffer{}
	            data := url.Values{}
	            data.Set("content", fmt.Sprintf("Could not find the file: %s", file))
	            _, err := body.Write([]byte(data.Encode()))
	            if err != nil {
		            fmt.Println("Error writing body:", err)
		            continue
	            }
                req, err := http.NewRequest("POST", followUpURL, body) 
                if err != nil {
		            fmt.Println("Error creating request:", err)
                    continue
	            }
	            req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	            client := &http.Client{}
	            resp, err := client.Do(req)
	            if err != nil {
		            fmt.Println("Error sending request:", err)
                    continue
	            }
	            defer resp.Body.Close()
                continue
            }
            sendFileToDiscord(followUpURL, fileBuffer, fileName)
        }
    }

    if rData.Data.Name == "upload" {
        for _, attachment := range rData.Data.Resolved.Attachments {
            res, err := http.Get(attachment.Url)
            if err != nil {
                return &events.APIGatewayProxyResponse{
                    StatusCode: 500,
                    Body:       "Error uploading file",
                }, nil
            }

            var buf bytes.Buffer
            _, err = io.Copy(&buf, res.Body)
            if err != nil {
                return &events.APIGatewayProxyResponse{
                    StatusCode: 500,
                    Body:       "Error uploading file",
                }, nil
            }
            defer res.Body.Close()

            err = uploadFile("nyi", bytes.NewReader(buf.Bytes()), attachment.Filename)
            if err != nil {
                return &events.APIGatewayProxyResponse{
                    StatusCode: 500,
                    Body:       "Error uploading file",
                }, nil
            }
        }

        res := Response{
            Type: 4,
        }

        res.Data.Content = "Successfully uploaded files!"
        responseBody, err := json.Marshal(res)
        if err != nil {
            return &events.APIGatewayProxyResponse{
                StatusCode: 500,
                Body:       "Unable to encode response JSON",
            }, nil
        }
        return &events.APIGatewayProxyResponse{
            StatusCode: 200,
            Body:       string(responseBody),
            Headers: map[string]string{
                "Content-Type": "application/json",
            },
        }, nil
    }
    return &events.APIGatewayProxyResponse{
        StatusCode: 400,
        Body:       "Bad request",
    }, nil
}

func main() {
    cfg, err := config.LoadDefaultConfig(
        context.TODO(),
        config.WithRegion("ap-southeast-1"),
    )
    if err != nil {
        log.Fatalf("unable to load SDK config, %v", err)
    }
    svc = s3.NewFromConfig(cfg)

    lambda.Start(rootHandler)
}