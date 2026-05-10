/**
 * SPDX-FileComment: MitM Aggregator Go Prototype
 * SPDX-FileType: SOURCE
 * SPDX-FileContributor: ZHENG Robert
 * SPDX-FileCopyrightText: 2026 ZHENG Robert
 * SPDX-License-Identifier: Apache-2.0
 *
 * @file main.go
 * @brief Go-based uploader prototype with logging and HTTP server.
 * @version 0.1.0
 * @date 2026-05-10
 *
 * @author ZHENG Robert (robert @hase-zheng.net)
 * @copyright Copyright (c) 2026 ZHENG Robert
 * @LICENSE Apache-2.0
 */
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"
)

// Config holds the application configuration
type Config struct {
	BaseURL    string
	Login      string
	Password   string
	UploadFile string
}

func loadEnv() Config {
	// Simple .env parser
	data, err := os.ReadFile(".env")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			}
		}
	}

	cfg := Config{
		BaseURL:    os.Getenv("CORITY_BASE_URL"),
		Login:      os.Getenv("CORITY_LOGIN"),
		Password:   os.Getenv("CORITY_PASSWORD"),
		UploadFile: os.Getenv("CORITY_UPLOAD_FILE"),
	}

	if cfg.UploadFile == "" {
		cfg.UploadFile = "data.json"
	}
	return cfg
}

func setupRedirection() {
	logFilename := time.Now().Format("2006-01-02") + ".log"
	logFile, err := os.OpenFile(logFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	// Redirect stdout and stderr using syscall (Linux/Unix specific)
	err = syscall.Dup2(int(logFile.Fd()), int(os.Stdout.Fd()))
	if err != nil {
		log.Printf("Failed to redirect stdout: %v", err)
	}
	err = syscall.Dup2(int(logFile.Fd()), int(os.Stderr.Fd()))
	if err != nil {
		log.Printf("Failed to redirect stderr: %v", err)
	}

	log.SetOutput(logFile)
	log.Printf("--- Session started at %v ---", time.Now())
}

func startHTTPServer() {
	port := "8080"
	fs := http.FileServer(http.Dir("."))
	http.Handle("/", fs)

	go func() {
		log.Printf("Serving HTTP on port %s", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatalf("HTTP Server failed: %v", err)
		}
	}()
}

func doJSONRequest(method, url string, headers map[string]string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	return respData, resp.StatusCode, err
}

func main() {
	setupRedirection()
	cfg := loadEnv()
	startHTTPServer()

	if cfg.BaseURL == "" || cfg.Login == "" || cfg.Password == "" {
		log.Println("Missing required configuration (BaseURL, Login, or Password)")
	} else {
		runUploadFlow(cfg)
	}

	log.Println("Upload process finished. Keeping HTTP server alive (Ctrl+C to stop).")
	// Block forever
	select {}
}

func runUploadFlow(cfg Config) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")

	// 1. Get Refresh Token
	log.Println("Authenticating (refresh token)...")
	refreshPayload := map[string]interface{}{
		"user": map[string]string{
			"LoginName":     cfg.Login,
			"Loginpassword": cfg.Password,
		},
	}
	headers := map[string]string{"Content-Type": "application/json"}

	respData, statusCode, err := doJSONRequest("POST", baseURL+"/api/refreshtoken", headers, refreshPayload)
	if err != nil {
		log.Printf("Failed to obtain refresh token: %v", err)
		return
	}
	if statusCode < 200 || statusCode >= 300 {
		log.Printf("Failed to obtain refresh token, status %d: %s", statusCode, string(respData))
		return
	}

	var refreshResult map[string]interface{}
	if err := json.Unmarshal(respData, &refreshResult); err != nil {
		log.Printf("Failed to parse refresh token response: %v", err)
		return
	}

	// Support 'Token' or 'token'
	var refreshToken string
	if val, ok := refreshResult["Token"].(string); ok {
		refreshToken = val
	} else if val, ok := refreshResult["token"].(string); ok {
		refreshToken = val
	}

	if refreshToken == "" {
		log.Println("Refresh token not found in response")
		return
	}
	log.Printf("Received refresh token (len=%d)", len(refreshToken))

	// 2. Get Access Token
	log.Println("Requesting access token...")
	authHeaders := map[string]string{
		"Authorization": "Bearer " + refreshToken,
	}

	respData, statusCode, err = doJSONRequest("GET", baseURL+"/api/token/", authHeaders, nil)
	if err != nil {
		log.Printf("Failed to obtain access token: %v", err)
		return
	}
	if statusCode < 200 || statusCode >= 300 {
		log.Printf("Failed to obtain access token, status %d: %s", statusCode, string(respData))
		return
	}

	var accessResult map[string]interface{}
	if err := json.Unmarshal(respData, &accessResult); err != nil {
		log.Printf("Failed to parse access token response: %v", err)
		return
	}

	var accessToken string
	if val, ok := accessResult["AccessToken"].(string); ok {
		accessToken = val
	} else if val, ok := accessResult["access_token"].(string); ok {
		accessToken = val
	} else if val, ok := accessResult["token"].(string); ok {
		accessToken = val
	}

	if accessToken == "" {
		log.Println("Access token not found in response")
		return
	}
	log.Printf("Received access token (len=%d)", len(accessToken))

	// 3. Upload Data
	fileData, err := os.ReadFile(cfg.UploadFile)
	if err != nil {
		log.Printf("Failed to read upload file: %v", err)
		return
	}

	var payload interface{}
	if err := json.Unmarshal(fileData, &payload); err != nil {
		log.Printf("Failed to parse upload file as JSON: %v", err)
		return
	}

	log.Printf("Uploading employee import to %s/api/employeeimport", baseURL)
	uploadHeaders := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + accessToken,
	}

	respData, statusCode, err = doJSONRequest("POST", baseURL+"/api/employeeimport", uploadHeaders, payload)
	if err != nil {
		log.Printf("Upload failed: %v", err)
		return
	}

	if statusCode < 200 || statusCode >= 300 {
		log.Printf("Upload failed, status %d: %s", statusCode, string(respData))
		return
	}

	log.Println("Upload successful. Server response:")
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, respData, "", "  "); err == nil {
		log.Println(prettyJSON.String())
		fmt.Println(prettyJSON.String()) // Will go to log file via redirection
	} else {
		log.Println(string(respData))
	}
}
