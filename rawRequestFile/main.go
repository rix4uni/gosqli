package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func main() {
	// Command-line flag to specify the raw request file
	rawRequestFile := flag.String("r", "", "Path to the Burp Suite raw request file")
	flag.Parse()

	if *rawRequestFile == "" {
		fmt.Println("Please provide the path to the raw request file using the -r flag.")
		return
	}

	// Open the raw request file
	file, err := os.Open(*rawRequestFile)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	// Read the file line by line
	scanner := bufio.NewScanner(file)

	// Variables to hold the request parts
	var method, url string
	var headers = make(map[string]string)
	var body string
	var isBody bool

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines (between headers and body)
		if line == "" {
			isBody = true
			continue
		}

		// First line is the request line (e.g., POST /path HTTP/1.1)
		if method == "" {
			parts := strings.Split(line, " ")
			if len(parts) < 2 {
				fmt.Println("Invalid request line:", line)
				return
			}
			method = parts[0]
			url = parts[1]
			continue
		}

		// Headers are before the body
		if !isBody {
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) == 2 {
				headers[parts[0]] = parts[1]
			}
		} else {
			// Body is after the headers
			body += line + "\n"
		}
	}

	// Remove trailing newline from body
	body = strings.TrimSuffix(body, "\n")

	// Print the request details
	fmt.Println("Request Method:", method)
	fmt.Println("Request URL:", url)
	fmt.Println("Request Headers:")
	for key, value := range headers {
		fmt.Printf("%s: %s\n", key, value)
	}
	fmt.Println("Request Body:")
	fmt.Println(body)
	fmt.Println("---------------------------")

	// Create the HTTP request
	client := &http.Client{}
	req, err := http.NewRequest(method, "http://"+headers["Host"]+url, bytes.NewBufferString(body))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	// Set the headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Send the request
	res, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer res.Body.Close()

	// Output the response status
	fmt.Println("Response Status:", res.Status)

	// Handle gzip-encoded response
	var responseBody []byte
	if res.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(res.Body)
		if err != nil {
			fmt.Println("Error creating gzip reader:", err)
			return
		}
		defer gzipReader.Close()
		responseBody, err = io.ReadAll(gzipReader)
		if err != nil {
			fmt.Println("Error reading gzip response body:", err)
			return
		}
	} else {
		responseBody, err = io.ReadAll(res.Body)
		if err != nil {
			fmt.Println("Error reading response body:", err)
			return
		}
	}

	// Print the decoded response body
	fmt.Println("Response Body:")
	fmt.Println(string(responseBody))
}
