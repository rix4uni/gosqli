package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"

	"github.com/rix4uni/gosqli/banner"
	"github.com/spf13/pflag"
)

// Declare package-level color functions
var Red = color.New(color.FgRed).SprintFunc()
var Green = color.New(color.FgGreen).SprintFunc()
var Yellow = color.New(color.FgYellow).SprintFunc()
var Magenta = color.New(color.FgMagenta).SprintFunc()
var Cyan = color.New(color.FgCyan).SprintFunc()

// Package-level mutex for file operations
var fileMutex sync.Mutex

// HTTPRequest represents a parsed HTTP request
type HTTPRequest struct {
	Method    string
	URL       string
	Headers   map[string]string
	Body      string
	UserAgent string
}

func fetchURL(ctx context.Context, cancel context.CancelFunc, url string, userAgent string, retries int) (int, string, float64, error) {
	return fetchURLWithRequest(ctx, cancel, url, userAgent, "", nil, retries)
}

func fetchURLWithRequest(ctx context.Context, cancel context.CancelFunc, targetURL string, userAgent string, method string, headers map[string]string, retries int, body ...string) (int, string, float64, error) {
	if headers == nil {
		headers = make(map[string]string)
	}
	var lastErr error
	var statusCode int
	var server string
	var responseTime float64

	// Custom HTTP Transport to disable HTTP/2
	transport := &http.Transport{
		TLSNextProto: make(map[string]func(string, *tls.Conn) http.RoundTripper),
	}
	client := &http.Client{Transport: transport}

	// Determine method
	if method == "" {
		method = "GET"
	}

	var requestBody *strings.Reader
	if len(body) > 0 && body[0] != "" {
		requestBody = strings.NewReader(body[0])
	}

	for attempt := 0; attempt <= retries; attempt++ {
		startTime := time.Now()

		var req *http.Request
		var err error
		if requestBody != nil {
			requestBody.Seek(0, 0) // Reset reader
			req, err = http.NewRequestWithContext(ctx, method, targetURL, requestBody)
		} else {
			req, err = http.NewRequestWithContext(ctx, method, targetURL, nil)
		}

		if err != nil {
			lastErr = err
			continue
		}

		// Set User-Agent
		if userAgent != "" {
			req.Header.Set("User-Agent", userAgent)
		}

		// Set custom headers
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() == context.Canceled {
				// If context is canceled, exit early
				return 0, "", 0, ctx.Err()
			}

			// Check if the error is a protocol error and cancel the context
			if strings.Contains(err.Error(), "PROTOCOL_ERROR") {
				fmt.Println("Protocol error detected, cancelling the request.")
				cancel() // Cancels the context
				return 0, "", 0, err
			}

			lastErr = err
			if attempt < retries {
				fmt.Printf(Yellow("RETRYING REQUEST: %s (attempt %d/%d)\n"), targetURL, attempt+1, retries)
				continue
			}
			return 0, "", 0, lastErr
		}
		defer resp.Body.Close()

		responseTime = time.Since(startTime).Seconds()
		server = resp.Header.Get("Server")
		statusCode = resp.StatusCode
		return statusCode, server, responseTime, nil
	}
	return statusCode, server, responseTime, lastErr
}

func verifyURL(ctx context.Context, cancel context.CancelFunc, url string, verifyCount int, responseFlag float64, verifyDelay float64, userAgent string, retries int, requiredCount int) (string, bool, error) {
	return verifyURLWithRequest(ctx, cancel, url, "", nil, "", verifyCount, responseFlag, verifyDelay, userAgent, retries, requiredCount)
}

func verifyURLWithRequest(ctx context.Context, cancel context.CancelFunc, targetURL string, method string, headers map[string]string, body string, verifyCount int, responseFlag float64, verifyDelay float64, userAgent string, retries int, requiredCount int) (string, bool, error) {
	var responseTimes []float64
	for i := 0; i < verifyCount; i++ {
		_, _, responseTime, err := fetchURLWithRequest(ctx, cancel, targetURL, userAgent, method, headers, retries, body)
		if err != nil {
			return "", false, err
		}
		responseTimes = append(responseTimes, responseTime)
		time.Sleep(time.Duration(verifyDelay) * time.Millisecond)
	}

	var countGreaterThanFlag int
	for _, rt := range responseTimes {
		if rt > responseFlag {
			countGreaterThanFlag++
		}
	}

	isVerified := requiredCount == 0 && len(responseTimes) > 0 && countGreaterThanFlag == len(responseTimes) || requiredCount > 0 && countGreaterThanFlag >= requiredCount

	var responseTimesStr []string
	for _, rt := range responseTimes {
		responseTimesStr = append(responseTimesStr, fmt.Sprintf("%.2f s", rt))
	}
	responseTimesSummary := strings.Join(responseTimesStr, ", ")

	return responseTimesSummary, isVerified, nil
}

func parseHTTPRequest(filepath string) (*HTTPRequest, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("error opening request file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading request file: %v", err)
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("request file is empty")
	}

	req := &HTTPRequest{
		Headers: make(map[string]string),
	}

	// Parse request line (first line)
	requestLine := lines[0]
	parts := strings.Fields(requestLine)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid request line: %s", requestLine)
	}

	req.Method = parts[0]
	path := parts[1]

	// Parse headers
	headerEnd := -1
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			headerEnd = i
			break
		}

		// Parse header line
		colonIdx := strings.Index(line, ":")
		if colonIdx > 0 {
			key := strings.TrimSpace(line[:colonIdx])
			value := strings.TrimSpace(line[colonIdx+1:])
			req.Headers[strings.ToLower(key)] = value

			// Extract User-Agent
			if strings.ToLower(key) == "user-agent" {
				req.UserAgent = value
			}

			// Extract Host to build full URL
			if strings.ToLower(key) == "host" {
				protocol := "http"
				// Check if HTTPS is indicated
				if strings.Contains(value, ":443") {
					protocol = "https"
				}
				// Check request line for protocol hint
				if strings.Contains(requestLine, "https://") {
					protocol = "https"
				}
				// Build full URL
				if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
					req.URL = path
				} else {
					req.URL = fmt.Sprintf("%s://%s%s", protocol, value, path)
				}
			}
		}
	}

	// If URL wasn't set from Host header, try to extract from request line
	if req.URL == "" {
		if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
			req.URL = path
		} else {
			// Try to find Host header
			host := req.Headers["host"]
			if host != "" {
				protocol := "http"
				// Check if HTTPS is indicated
				if strings.Contains(host, ":443") || strings.Contains(requestLine, "https://") {
					protocol = "https"
				}
				req.URL = fmt.Sprintf("%s://%s%s", protocol, host, path)
			} else {
				return nil, fmt.Errorf("could not determine URL from request")
			}
		}
	}

	// Parse body (everything after empty line)
	if headerEnd >= 0 && headerEnd+1 < len(lines) {
		req.Body = strings.Join(lines[headerEnd+1:], "\n")
	}

	// Set default User-Agent if not found
	if req.UserAgent == "" {
		req.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36"
	}

	return req, nil
}

// replaceInjectionMarker replaces * with payload in URL, headers, and body
func replaceInjectionMarker(req *HTTPRequest, payload string) (*HTTPRequest, error) {
	newReq := &HTTPRequest{
		Method:    req.Method,
		URL:       strings.Replace(req.URL, "*", payload, -1),
		Headers:   make(map[string]string),
		Body:      strings.Replace(req.Body, "*", payload, -1),
		UserAgent: req.UserAgent,
	}

	// Copy and replace in headers
	for key, value := range req.Headers {
		newReq.Headers[key] = strings.Replace(value, "*", payload, -1)
	}

	return newReq, nil
}

// isDirectory checks if the given path is a directory
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// getRequestFiles returns all request files from a directory
func getRequestFiles(dirPath string) ([]string, error) {
	var files []string

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("error reading directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			// Include all files, or filter by extension if needed
			filePath := filepath.Join(dirPath, entry.Name())
			files = append(files, filePath)
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("directory is empty or contains no files")
	}

	return files, nil
}

// getConfigDir returns the gosqli config directory path and creates it if it doesn't exist
func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error getting home directory: %v", err)
	}
	configDir := filepath.Join(homeDir, ".config", "gosqli")
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		return "", fmt.Errorf("error creating config directory: %v", err)
	}
	return configDir, nil
}

// saveConfirmedURL saves both URL versions: modifiedURL (with payload) to burpsuite file and originalURL (with * marker) to sqlmap_ghauri file
func saveConfirmedURL(modifiedURL string, originalURL string) error {
	fileMutex.Lock()
	defer fileMutex.Unlock()

	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	// Save modified URL with payload to burpsuite file
	burpsuiteFile := filepath.Join(configDir, "sqliconfirmed.burpsuite")
	file, err := os.OpenFile(burpsuiteFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening burpsuite output file: %v", err)
	}
	_, err = file.WriteString(modifiedURL + "\n")
	if err != nil {
		file.Close()
		return fmt.Errorf("error writing to burpsuite output file: %v", err)
	}
	file.Close()

	// Save original URL with * marker to sqlmap_ghauri file
	sqlmapFile := filepath.Join(configDir, "sqliconfirmed.sqlmap_ghauri")
	file, err = os.OpenFile(sqlmapFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening sqlmap_ghauri output file: %v", err)
	}
	_, err = file.WriteString(originalURL + "\n")
	if err != nil {
		file.Close()
		return fmt.Errorf("error writing to sqlmap_ghauri output file: %v", err)
	}
	file.Close()

	return nil
}

// httpRequestToRaw converts an HTTPRequest to raw HTTP request string format
func httpRequestToRaw(req *HTTPRequest) string {
	var raw strings.Builder

	// Parse URL to get path and query
	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		// Fallback if URL parsing fails
		raw.WriteString(fmt.Sprintf("%s %s HTTP/1.1\n", req.Method, req.URL))
	} else {
		path := parsedURL.Path
		if parsedURL.RawQuery != "" {
			path += "?" + parsedURL.RawQuery
		}
		if path == "" {
			path = "/"
		}
		raw.WriteString(fmt.Sprintf("%s %s HTTP/1.1\n", req.Method, path))
	}

	// Write headers (capitalize first letter of each word for standard HTTP format)
	for key, value := range req.Headers {
		// Capitalize header name properly (e.g., "content-type" -> "Content-Type")
		parts := strings.Split(key, "-")
		for i, part := range parts {
			if len(part) > 0 {
				parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
			}
		}
		headerName := strings.Join(parts, "-")
		raw.WriteString(fmt.Sprintf("%s: %s\n", headerName, value))
	}

	// Ensure User-Agent is included
	if req.UserAgent != "" && req.Headers["user-agent"] == "" {
		raw.WriteString(fmt.Sprintf("User-Agent: %s\n", req.UserAgent))
	}

	// Ensure Host header is included
	if parsedURL != nil && parsedURL.Host != "" && req.Headers["host"] == "" {
		raw.WriteString(fmt.Sprintf("Host: %s\n", parsedURL.Host))
	}

	// Empty line before body
	raw.WriteString("\n")

	// Write body if present
	if req.Body != "" {
		raw.WriteString(req.Body)
	}

	return raw.String()
}

// saveConfirmedRequest saves a confirmed SQLI HTTP request to the appropriate directory
// Returns the saved file path for sqlmap/ghauri directory (when withPayload is false)
func saveConfirmedRequest(req *HTTPRequest, originalReq *HTTPRequest, filename string, withPayload bool) (string, error) {
	fileMutex.Lock()
	defer fileMutex.Unlock()

	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}

	var targetDir string
	var reqToSave *HTTPRequest

	if withPayload {
		// For BurpSuite: save request with actual payload
		targetDir = filepath.Join(configDir, "sqliconfirmed_request", "burpsuite")
		reqToSave = req
	} else {
		// For sqlmap/ghauri: save request with * marker
		targetDir = filepath.Join(configDir, "sqliconfirmed_request", "sqlmap_ghauri")
		reqToSave = originalReq
	}

	// Create target directory if it doesn't exist
	err = os.MkdirAll(targetDir, 0755)
	if err != nil {
		return "", fmt.Errorf("error creating target directory: %v", err)
	}

	// Generate unique filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	baseName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".txt"
	}
	outputFilename := fmt.Sprintf("%s_%s%s", baseName, timestamp, ext)
	outputPath := filepath.Join(targetDir, outputFilename)

	// Convert request to raw format and save
	rawRequest := httpRequestToRaw(reqToSave)
	err = os.WriteFile(outputPath, []byte(rawRequest), 0644)
	if err != nil {
		return "", fmt.Errorf("error writing request file: %v", err)
	}

	// Return the saved file path if it's for sqlmap/ghauri, empty string otherwise
	if !withPayload {
		return outputPath, nil
	}
	return "", nil
}

// getLogFilePath generates a log file path with timestamp
func getLogFilePath(tool string) string {
	configDir, err := getConfigDir()
	if err != nil {
		return ""
	}
	logsDir := filepath.Join(configDir, "logs")
	os.MkdirAll(logsDir, 0755)
	timestamp := time.Now().Format("20060102_150405")
	return filepath.Join(logsDir, fmt.Sprintf("%s_%s.log", tool, timestamp))
}

// launchSqlmap launches sqlmap as a background process
func launchSqlmap(target string, isRequestFile bool, logFile string) error {
	var args []string
	if isRequestFile {
		args = []string{"-r", target, "--random-agent", "--level", "5", "--risk", "3", "--ignore-code=500", "--dbs", "-time-sec=12", "--batch", "--flush-session"}
	} else {
		args = []string{"-u", target, "--random-agent", "--level", "5", "--risk", "3", "--ignore-code=500", "--dbs", "-time-sec=12", "--batch", "--flush-session"}
	}

	logFileHandle, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("error creating log file: %v", err)
	}

	// Write header to log file indicating the target
	// For request files, show just the filename; for URLs, show the full URL
	headerTarget := target
	if isRequestFile {
		headerTarget = filepath.Base(target)
	}
	header := fmt.Sprintf("URL_FILE: %s\n\n", headerTarget)
	_, err = logFileHandle.WriteString(header)
	if err != nil {
		logFileHandle.Close()
		return fmt.Errorf("error writing header to log file: %v", err)
	}

	// Don't close the file handle - let the process manage it
	// The file will be closed when the process exits

	cmd := exec.Command("sqlmap", args...)
	cmd.Stdout = logFileHandle
	cmd.Stderr = logFileHandle

	err = cmd.Start()
	if err != nil {
		logFileHandle.Close()
		return fmt.Errorf("error starting sqlmap: %v", err)
	}

	// Detach from the process - don't wait for it
	go func() {
		cmd.Wait()
		logFileHandle.Close()
	}()

	return nil
}

// launchGhauri launches ghauri as a background process
func launchGhauri(target string, isRequestFile bool, logFile string) error {
	var args []string
	if isRequestFile {
		args = []string{"-r", target, "--level", "3", "--dbs", "--time-sec", "12", "--batch", "--flush-session"}
	} else {
		args = []string{"-u", target, "--level", "3", "--dbs", "--time-sec", "12", "--batch", "--flush-session"}
	}

	logFileHandle, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("error creating log file: %v", err)
	}

	// Write header to log file indicating the target
	// For request files, show just the filename; for URLs, show the full URL
	headerTarget := target
	if isRequestFile {
		headerTarget = filepath.Base(target)
	}
	header := fmt.Sprintf("URL_FILE: %s\n\n", headerTarget)
	_, err = logFileHandle.WriteString(header)
	if err != nil {
		logFileHandle.Close()
		return fmt.Errorf("error writing header to log file: %v", err)
	}

	// Don't close the file handle - let the process manage it
	// The file will be closed when the process exits

	cmd := exec.Command("ghauri", args...)
	cmd.Stdout = logFileHandle
	cmd.Stderr = logFileHandle

	err = cmd.Start()
	if err != nil {
		logFileHandle.Close()
		return fmt.Errorf("error starting ghauri: %v", err)
	}

	// Detach from the process - don't wait for it
	go func() {
		cmd.Wait()
		logFileHandle.Close()
	}()

	return nil
}

// launchExploitation launches the appropriate exploitation tool(s) based on the tool parameter
func launchExploitation(target string, isRequestFile bool, tool string) error {
	tool = strings.ToLower(tool)

	switch tool {
	case "sqlmap":
		logFile := getLogFilePath("sqlmap")
		if logFile == "" {
			return fmt.Errorf("error getting log file path")
		}
		err := launchSqlmap(target, isRequestFile, logFile)
		if err != nil {
			return err
		}
		fmt.Printf(Cyan("Started sqlmap exploitation in background. Log: %s\n"), logFile)
		return nil

	case "ghauri":
		logFile := getLogFilePath("ghauri")
		if logFile == "" {
			return fmt.Errorf("error getting log file path")
		}
		err := launchGhauri(target, isRequestFile, logFile)
		if err != nil {
			return err
		}
		fmt.Printf(Cyan("Started ghauri exploitation in background. Log: %s\n"), logFile)
		return nil

	case "both":
		// Launch both tools
		sqlmapLogFile := getLogFilePath("sqlmap")
		if sqlmapLogFile == "" {
			return fmt.Errorf("error getting sqlmap log file path")
		}
		err := launchSqlmap(target, isRequestFile, sqlmapLogFile)
		if err != nil {
			fmt.Printf(Yellow("Warning: Failed to start sqlmap: %v\n"), err)
		} else {
			fmt.Printf(Cyan("Started sqlmap exploitation in background. Log: %s\n"), sqlmapLogFile)
		}

		ghauriLogFile := getLogFilePath("ghauri")
		if ghauriLogFile == "" {
			return fmt.Errorf("error getting ghauri log file path")
		}
		err = launchGhauri(target, isRequestFile, ghauriLogFile)
		if err != nil {
			fmt.Printf(Yellow("Warning: Failed to start ghauri: %v\n"), err)
		} else {
			fmt.Printf(Cyan("Started ghauri exploitation in background. Log: %s\n"), ghauriLogFile)
		}
		return nil

	default:
		return fmt.Errorf("invalid tool: %s (must be sqlmap, ghauri, or both)", tool)
	}
}

func sendProxyRequest(ctx context.Context, targetURL string, userAgent string, proxyURL string, httpReq *HTTPRequest, filename string, server string, responseTimesSummary string) {
	if proxyURL == "" {
		return
	}

	proxyParsed, err := url.Parse(proxyURL)
	if err != nil {
		fmt.Printf(Yellow("Warning: Invalid proxy URL: %s\n"), err)
		return
	}

	// Custom HTTP Transport with proxy and disable HTTP/2
	transport := &http.Transport{
		Proxy:        http.ProxyURL(proxyParsed),
		TLSNextProto: make(map[string]func(string, *tls.Conn) http.RoundTripper),
	}
	client := &http.Client{Transport: transport}

	// Determine method and body
	method := "GET"
	var requestBody *strings.Reader
	if httpReq != nil {
		method = httpReq.Method
		if method == "" {
			method = "GET"
		}
		if httpReq.Body != "" {
			requestBody = strings.NewReader(httpReq.Body)
		}
	}

	// Create request
	var req *http.Request
	if requestBody != nil {
		req, err = http.NewRequestWithContext(ctx, method, targetURL, requestBody)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, targetURL, nil)
	}
	if err != nil {
		fmt.Printf(Yellow("Warning: Failed to create proxy request: %s\n"), err)
		return
	}

	// Set headers
	if httpReq != nil {
		// Set all headers from the HTTP request
		for key, value := range httpReq.Headers {
			req.Header.Set(key, value)
		}
		// Ensure User-Agent is set (it might not be in Headers if it was defaulted)
		if httpReq.UserAgent != "" {
			req.Header.Set("User-Agent", httpReq.UserAgent)
		}
	} else {
		// Fallback to simple User-Agent only for backward compatibility
		req.Header.Set("User-Agent", userAgent)
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf(Yellow("Warning: Proxy request failed: %s\n"), err)
		return
	}
	defer resp.Body.Close()

	// Build the output message with optional fields
	var parts []string
	if filename != "" {
		parts = append(parts, fmt.Sprintf("[%s]", filename))
	}
	parts = append(parts, targetURL)
	parts = append(parts, fmt.Sprintf("[%d]", resp.StatusCode))
	if server != "" {
		parts = append(parts, fmt.Sprintf("[%s]", server))
	}
	if responseTimesSummary != "" {
		parts = append(parts, fmt.Sprintf("[%s]", responseTimesSummary))
	}
	outputMsg := strings.Join(parts, " ")
	fmt.Printf(Cyan("Proxy request sent: %s\n"), outputMsg)
}

func processURL(ctx context.Context, cancel context.CancelFunc, url string, payloads []string, responseFlag, verify, verifyDelay, retries int, noColor bool, userAgent string, stop int, wg *sync.WaitGroup, mu *sync.Mutex, stopOnce *sync.Once, maxConcurrency int, requiredCount int, proxy string, output bool, onConfirmed string) {
	defer wg.Done()

	sqlFoundCount := 0 // Reset for each URL

	statusCode, server, responseTime, err := fetchURL(ctx, cancel, url, userAgent, retries)
	if err != nil {
		fmt.Println("Error fetching the URL:", err)
		return
	}
	nStarURL := strings.Replace(url, "*", "", -1)
	fmt.Printf(Yellow("NORMAL REQUEST: %s [%d] [%s] [%.2f s]\n"), nStarURL, statusCode, server, responseTime)

	var payloadWg sync.WaitGroup
	payloadSem := make(chan struct{}, maxConcurrency)

	for _, payload := range payloads {
		select {
		case <-ctx.Done():
			fmt.Println(Cyan("Stopping further payloads due to context cancellation."))
			return
		default:
			payloadSem <- struct{}{}
			payloadWg.Add(1)
			go func(payload string) {
				defer func() { <-payloadSem }()
				defer payloadWg.Done()

				// Replace ADDTIME in the payload with 10
				payload = strings.Replace(payload, "ADDTIME", "10", -1)

				modifiedURL := strings.Replace(url, "*", payload, -1)
				statusCode, server, responseTime, err := fetchURL(ctx, cancel, modifiedURL, userAgent, retries)
				if err != nil {
					if ctx.Err() == context.Canceled {
						// Skip further processing if context is canceled
						return
					}
					fmt.Println("Error fetching the URL:", err)
					return
				}

				if responseTime > float64(responseFlag) {
					if noColor {
						fmt.Printf("SQLI FOUND: %s [%d] [%s] [%.2f s]\n", modifiedURL, statusCode, server, responseTime)
					} else {
						fmt.Printf(Red("SQLI FOUND: %s [%d] [%s] [%.2f s]\n"), modifiedURL, statusCode, server, responseTime)
					}

					if verify > 1 {
						responseTimesSummary, isVerified, err := verifyURL(ctx, cancel, modifiedURL, verify, float64(responseFlag), float64(verifyDelay), userAgent, retries, requiredCount)
						if err != nil {
							if ctx.Err() == context.Canceled {
								// Skip further processing if context is canceled
								return
							}
							fmt.Println("Error verifying the URL:", err)
							return
						}
						if isVerified {
							mu.Lock()
							defer mu.Unlock()

							select {
							case <-ctx.Done():
								return
							default:
								if noColor {
									fmt.Printf("SQLI CONFIRMED: %s [%d] [%s] [%s]\n", modifiedURL, statusCode, server, responseTimesSummary)
								} else {
									fmt.Printf(Red("SQLI CONFIRMED: %s [%d] [%s] [%s]\n"), modifiedURL, statusCode, server, responseTimesSummary)
								}

								// Send request through proxy if configured
								sendProxyRequest(ctx, modifiedURL, userAgent, proxy, nil, "", server, responseTimesSummary)

								// Save confirmed SQLI URL if output flag is enabled
								if output {
									if err := saveConfirmedURL(modifiedURL, url); err != nil {
										fmt.Printf(Yellow("Warning: Failed to save confirmed URL: %v\n"), err)
									}
								}

								// Launch exploitation tool if on-confirmed flag is set
								if onConfirmed != "" && onConfirmed != "none" {
									if err := launchExploitation(modifiedURL, false, onConfirmed); err != nil {
										fmt.Printf(Yellow("Warning: Failed to launch exploitation: %v\n"), err)
									}
								}

								sqlFoundCount++ // No need to dereference
								if stop > 0 && sqlFoundCount >= stop {
									fmt.Println(Cyan("Stopping further checks for this URL due to stop flag."))
									stopOnce.Do(cancel)
									return
								}
							}
						} else {
							fmt.Printf(Green("SQLI FP CONFIRMED: %s [%d] [%s] [%s]\n"), modifiedURL, statusCode, server, responseTimesSummary)
						}
					}
				} else {
					fmt.Printf(Green("NOT FOUND: %s [%d] [%s] [%.2f s]\n"), modifiedURL, statusCode, server, responseTime)
				}
			}(payload)
		}
	}
	payloadWg.Wait()
}

func processHTTPRequest(ctx context.Context, cancel context.CancelFunc, httpReq *HTTPRequest, payloads []string, responseFlag, verify, verifyDelay, retries int, noColor bool, stop int, wg *sync.WaitGroup, mu *sync.Mutex, stopOnce *sync.Once, maxConcurrency int, requiredCount int, proxy string, filename string, output bool, onConfirmed string) {
	defer wg.Done()

	sqlFoundCount := 0

	// Make baseline request
	statusCode, server, responseTime, err := fetchURLWithRequest(ctx, cancel, httpReq.URL, httpReq.UserAgent, httpReq.Method, httpReq.Headers, retries, httpReq.Body)
	if err != nil {
		fmt.Println("Error fetching the URL:", err)
		return
	}
	nStarURL := strings.Replace(httpReq.URL, "*", "", -1)
	fmt.Printf(Yellow("NORMAL REQUEST: [%s] %s [%d] [%s] [%.2f s]\n"), filename, nStarURL, statusCode, server, responseTime)

	var payloadWg sync.WaitGroup
	payloadSem := make(chan struct{}, maxConcurrency)

	for _, payload := range payloads {
		select {
		case <-ctx.Done():
			fmt.Println(Cyan("Stopping further payloads due to context cancellation."))
			return
		default:
			payloadSem <- struct{}{}
			payloadWg.Add(1)
			go func(payload string) {
				defer func() { <-payloadSem }()
				defer payloadWg.Done()

				// Replace ADDTIME in the payload with 10
				payload = strings.Replace(payload, "ADDTIME", "10", -1)

				// Replace * with payload in request
				modifiedReq, err := replaceInjectionMarker(httpReq, payload)
				if err != nil {
					fmt.Println("Error modifying request:", err)
					return
				}

				statusCode, server, responseTime, err := fetchURLWithRequest(ctx, cancel, modifiedReq.URL, modifiedReq.UserAgent, modifiedReq.Method, modifiedReq.Headers, retries, modifiedReq.Body)
				if err != nil {
					if ctx.Err() == context.Canceled {
						return
					}
					fmt.Println("Error fetching the URL:", err)
					return
				}

				if responseTime > float64(responseFlag) {
					if noColor {
						fmt.Printf("SQLI FOUND: [%s] %s [%d] [%s] [%.2f s]\n", filename, modifiedReq.URL, statusCode, server, responseTime)
					} else {
						fmt.Printf(Red("SQLI FOUND: [%s] %s [%d] [%s] [%.2f s]\n"), filename, modifiedReq.URL, statusCode, server, responseTime)
					}

					if verify > 1 {
						responseTimesSummary, isVerified, err := verifyURLWithRequest(ctx, cancel, modifiedReq.URL, modifiedReq.Method, modifiedReq.Headers, modifiedReq.Body, verify, float64(responseFlag), float64(verifyDelay), modifiedReq.UserAgent, retries, requiredCount)
						if err != nil {
							if ctx.Err() == context.Canceled {
								return
							}
							fmt.Println("Error verifying the URL:", err)
							return
						}
						if isVerified {
							mu.Lock()
							defer mu.Unlock()

							select {
							case <-ctx.Done():
								return
							default:
								if noColor {
									fmt.Printf("SQLI CONFIRMED: [%s] %s [%d] [%s] [%s]\n", filename, modifiedReq.URL, statusCode, server, responseTimesSummary)
								} else {
									fmt.Printf(Red("SQLI CONFIRMED: [%s] %s [%d] [%s] [%s]\n"), filename, modifiedReq.URL, statusCode, server, responseTimesSummary)
								}

								// Send request through proxy if configured
								sendProxyRequest(ctx, modifiedReq.URL, modifiedReq.UserAgent, proxy, modifiedReq, filename, server, responseTimesSummary)

								// Save confirmed SQLI request to files if output flag is enabled or on-confirmed is set
								var requestFilePath string
								if output {
									// Save request with payload for BurpSuite
									_, err := saveConfirmedRequest(modifiedReq, httpReq, filename, true)
									if err != nil {
										fmt.Printf(Yellow("Warning: Failed to save BurpSuite request: %v\n"), err)
									}
									// Save request with * marker for sqlmap/ghauri
									requestFilePath, err = saveConfirmedRequest(modifiedReq, httpReq, filename, false)
									if err != nil {
										fmt.Printf(Yellow("Warning: Failed to save sqlmap/ghauri request: %v\n"), err)
									}
								} else if onConfirmed != "" && onConfirmed != "none" {
									// Save request file for exploitation even if output flag is not set
									var err error
									requestFilePath, err = saveConfirmedRequest(modifiedReq, httpReq, filename, false)
									if err != nil {
										fmt.Printf(Yellow("Warning: Failed to save request file for exploitation: %v\n"), err)
									}
								}

								// Launch exploitation tool if on-confirmed flag is set
								if onConfirmed != "" && onConfirmed != "none" {
									// Use request file path if available, otherwise use URL
									if requestFilePath != "" {
										if err := launchExploitation(requestFilePath, true, onConfirmed); err != nil {
											fmt.Printf(Yellow("Warning: Failed to launch exploitation: %v\n"), err)
										}
									} else {
										// Fallback to URL if request file wasn't saved
										if err := launchExploitation(modifiedReq.URL, false, onConfirmed); err != nil {
											fmt.Printf(Yellow("Warning: Failed to launch exploitation: %v\n"), err)
										}
									}
								}

								sqlFoundCount++
								if stop > 0 && sqlFoundCount >= stop {
									fmt.Println(Cyan("Stopping further checks for this URL due to stop flag."))
									stopOnce.Do(cancel)
									return
								}
							}
						} else {
							fmt.Printf(Green("SQLI FP CONFIRMED: [%s] %s [%d] [%s] [%s]\n"), filename, modifiedReq.URL, statusCode, server, responseTimesSummary)
						}
					}
				} else {
					fmt.Printf(Green("NOT FOUND: [%s] %s [%d] [%s] [%.2f s]\n"), filename, modifiedReq.URL, statusCode, server, responseTime)
				}
			}(payload)
		}
	}
	payloadWg.Wait()
}

// Display flag values at the start of the program
func PrintInfo(responseFlag, verify, requiredCount, verifyDelay, retries, stop, maxParallel, maxConcurrency int) {
	fmt.Println("-------------------------------------------")
	fmt.Printf(" :: responseFlag    : %d\n", responseFlag)
	fmt.Printf(" :: verify          : %d\n", verify)
	fmt.Printf(" :: requiredCount   : %d\n", requiredCount)
	fmt.Printf(" :: verifyDelay     : %d\n", verifyDelay)
	fmt.Printf(" :: retries         : %d\n", retries)
	fmt.Printf(" :: stop            : %d\n", stop)
	fmt.Printf(" :: maxParallel     : %d\n", maxParallel)
	fmt.Printf(" :: maxConcurrency  : %d\n", maxConcurrency)
	fmt.Println("-------------------------------------------")
}

func main() {
	url := pflag.StringP("url", "u", "", "URL to fetch")
	list := pflag.StringP("list", "l", "", "File containing list of URLs")
	payloadFile := pflag.StringP("payload", "p", "", "File containing payloads")
	responseFlag := pflag.IntP("mrt", "m", 10, "Match response time with specified response time in seconds.")
	verify := pflag.IntP("verify", "v", 3, "Number of times to verify \"SQLI FOUND\".")
	requiredCount := pflag.IntP("requiredCount", "c", 0, "Number of response times greater than responseFlag required for SQLI CONFIRMED (0 means all).")
	verifyDelay := pflag.IntP("verifydelay", "d", 12000, "Delay in milliseconds between verify attempts.")
	retries := pflag.Int("retries", 0, "Number of retry attempts for failed HTTP requests.")
	noColor := pflag.Bool("no-color", false, "Do not save colored output.")
	stop := pflag.Int("stop", 1, "Stop checking pending HTTP requests after [stop] (0: means check all).")
	userAgent := pflag.String("H", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36", "Custom User-Agent header for HTTP requests.")
	maxParallel := pflag.IntP("parallel", "P", 1, "Maximum number of URLs Scan Parallely.")
	maxConcurrency := pflag.Int("concurrency", 20, "Maximum number of Payloads Scan concurrent.")
	silent := pflag.Bool("silent", false, "Silent mode.")
	versionFlag := pflag.Bool("version", false, "Print the version of the tool and exit.")
	proxy := pflag.String("proxy", "", "Proxy URL. E.g. --proxy http://127.0.0.1:8080")
	requestFile := pflag.StringP("request", "r", "", "Load HTTP request from a file")
	output := pflag.BoolP("output", "o", false, "Save SQLI CONFIRMED results to files")
	onConfirmed := pflag.String("on-confirmed", "ghauri", "Tool to use for exploitation: sqlmap, ghauri, both, or ghauri (default)")
	pflag.Parse()

	if *versionFlag {
		banner.PrintBanner()
		banner.PrintVersion()
		return
	}

	if !*silent {
		banner.PrintBanner()
		PrintInfo(*responseFlag, *verify, *requiredCount, *verifyDelay, *retries, *stop, *maxParallel, *maxConcurrency)
	}

	if *requiredCount > *verify {
		fmt.Println(Red("Error: -requiredCount flag value cannot be greater than -verify flag value."))
		os.Exit(1)
	}

	var payloads []string
	if *payloadFile != "" {
		file, err := os.Open(*payloadFile)
		if err != nil {
			fmt.Println("Error opening the payload file:", err)
			return
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			payloads = append(payloads, scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading the payload file:", err)
			return
		}
	}

	// Calculate total combinations
	var totalCombinations int
	if *url != "" {
		countStars := strings.Count(*url, "*")
		totalCombinations = countStars * len(payloads)
		fmt.Printf(Cyan("URLs Will be Scanning with * [%d]\n\n"), totalCombinations)
	} else if *list != "" {
		file, err := os.Open(*list)
		if err != nil {
			fmt.Println("Error opening the file:", err)
			return
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			url := scanner.Text()
			countStars := strings.Count(url, "*")
			totalCombinations += countStars * len(payloads)
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading the file:", err)
			return
		}
		fmt.Printf(Cyan("URLs Will be Scanning with * [%d]\n\n"), totalCombinations)
	} else if *requestFile != "" {
		// Will calculate after parsing request
	}

	var mu sync.Mutex
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sqlFoundCount := 0
	stopOnce := &sync.Once{}

	// Handle request file mode
	if *requestFile != "" {
		if len(payloads) == 0 {
			fmt.Println(Red("Error: -payload flag is required when using -r flag"))
			os.Exit(1)
		}

		// Check if path is directory or file
		if isDirectory(*requestFile) {
			// Handle directory: process all files in parallel
			requestFiles, err := getRequestFiles(*requestFile)
			if err != nil {
				fmt.Println(Red("Error reading directory:"), err)
				os.Exit(1)
			}

			// Calculate total combinations for all files
			totalCombinations = 0
			for _, filePath := range requestFiles {
				httpReq, err := parseHTTPRequest(filePath)
				if err != nil {
					fmt.Printf(Yellow("Warning: Skipping invalid request file %s: %v\n"), filePath, err)
					continue
				}

				// Check if request contains injection marker
				hasMarker := strings.Contains(httpReq.URL, "*") || strings.Contains(httpReq.Body, "*")
				for _, value := range httpReq.Headers {
					if strings.Contains(value, "*") {
						hasMarker = true
						break
					}
				}

				if hasMarker {
					countStars := strings.Count(httpReq.URL, "*") + strings.Count(httpReq.Body, "*")
					for _, value := range httpReq.Headers {
						countStars += strings.Count(value, "*")
					}
					totalCombinations += countStars * len(payloads)
				}
			}

			if totalCombinations > 0 {
				fmt.Printf(Cyan("Requests Will be Scanning with * [%d] from %d files\n\n"), totalCombinations, len(requestFiles))
			} else {
				fmt.Println(Red("Error: No valid request files with injection markers found in directory"))
				os.Exit(1)
			}

			// Process all files in parallel
			var wg sync.WaitGroup
			sem := make(chan struct{}, *maxParallel)

			for _, filePath := range requestFiles {
				sem <- struct{}{}
				wg.Add(1)
				go func(filePath string) {
					defer func() { <-sem }()

					httpReq, err := parseHTTPRequest(filePath)
					if err != nil {
						fmt.Printf(Yellow("Warning: Skipping invalid request file %s: %v\n"), filePath, err)
						wg.Done()
						return
					}

					// Check if request contains injection marker
					hasMarker := strings.Contains(httpReq.URL, "*") || strings.Contains(httpReq.Body, "*")
					for _, value := range httpReq.Headers {
						if strings.Contains(value, "*") {
							hasMarker = true
							break
						}
					}

					if !hasMarker {
						fmt.Printf(Cyan("Skipping request file (No * found): %s\n"), filePath)
						wg.Done()
						return
					}

					// Create a new context and cancel function for each request file
					ctx, cancel := context.WithCancel(context.Background())
					stopOnce := &sync.Once{} // Reset stopOnce for each request file
					filename := filepath.Base(filePath)
					processHTTPRequest(ctx, cancel, httpReq, payloads, *responseFlag, *verify, *verifyDelay, *retries, *noColor, *stop, &wg, &mu, stopOnce, *maxConcurrency, *requiredCount, *proxy, filename, *output, *onConfirmed)
				}(filePath)
			}
			wg.Wait()
			return
		} else {
			// Handle single file (existing behavior)
			httpReq, err := parseHTTPRequest(*requestFile)
			if err != nil {
				fmt.Println(Red("Error parsing request file:"), err)
				os.Exit(1)
			}

			// Check if request contains injection marker
			hasMarker := strings.Contains(httpReq.URL, "*") || strings.Contains(httpReq.Body, "*")
			for _, value := range httpReq.Headers {
				if strings.Contains(value, "*") {
					hasMarker = true
					break
				}
			}

			if !hasMarker {
				fmt.Println(Red("Error: Request file does not contain injection marker (*)"))
				os.Exit(1)
			}

			// Calculate total combinations
			countStars := strings.Count(httpReq.URL, "*") + strings.Count(httpReq.Body, "*")
			for _, value := range httpReq.Headers {
				countStars += strings.Count(value, "*")
			}
			totalCombinations = countStars * len(payloads)
			if totalCombinations > 0 {
				fmt.Printf(Cyan("Request Will be Scanning with * [%d]\n\n"), totalCombinations)
			}

			var wg sync.WaitGroup
			wg.Add(1)
			filename := filepath.Base(*requestFile)
			processHTTPRequest(ctx, cancel, httpReq, payloads, *responseFlag, *verify, *verifyDelay, *retries, *noColor, *stop, &wg, &mu, stopOnce, *maxConcurrency, *requiredCount, *proxy, filename, *output, *onConfirmed)
			wg.Wait()
			return
		}
	}

	if *url != "" {
		if strings.Contains(*url, "*") {
			statusCode, server, responseTime, err := fetchURL(ctx, cancel, *url, *userAgent, *retries)
			if err != nil {
				fmt.Println("Error fetching the URL:", err)
				return
			}
			nStarURL := strings.Replace(*url, "*", "", -1)
			fmt.Printf(Yellow("NORMAL REQUEST: %s [%d] [%s] [%.2f s]\n"), nStarURL, statusCode, server, responseTime)

			var payloadWg sync.WaitGroup
			payloadSem := make(chan struct{}, *maxConcurrency)

			for _, payload := range payloads {
				select {
				case <-ctx.Done():
					fmt.Println(Cyan("Stopping further payloads due to context cancellation."))
					return
				default:
					payloadSem <- struct{}{}
					payloadWg.Add(1)
					go func(payload string) {
						defer func() { <-payloadSem }()
						defer payloadWg.Done()

						// Replace ADDTIME in the payload with 10
						payload = strings.Replace(payload, "ADDTIME", "10", -1)

						modifiedURL := strings.Replace(*url, "*", payload, -1)
						statusCode, server, responseTime, err := fetchURL(ctx, cancel, modifiedURL, *userAgent, *retries)
						if err != nil {
							fmt.Println("Error fetching the URL:", err)
							return
						}

						if responseTime > float64(*responseFlag) {
							if *noColor {
								fmt.Printf("SQLI FOUND: %s [%d] [%s] [%.2f s]\n", modifiedURL, statusCode, server, responseTime)
							} else {
								fmt.Printf(Red("SQLI FOUND: %s [%d] [%s] [%.2f s]\n"), modifiedURL, statusCode, server, responseTime)
							}

							if *verify > 1 {
								responseTimesSummary, isVerified, err := verifyURL(ctx, cancel, modifiedURL, *verify, float64(*responseFlag), float64(*verifyDelay), *userAgent, *retries, *requiredCount)
								if err != nil {
									fmt.Println("Error verifying the URL:", err)
									return
								}
								if isVerified {
									mu.Lock()
									defer mu.Unlock()

									select {
									case <-ctx.Done():
										return
									default:
										if *noColor {
											fmt.Printf("SQLI CONFIRMED: %s [%d] [%s] [%s]\n", modifiedURL, statusCode, server, responseTimesSummary)
										} else {
											fmt.Printf(Red("SQLI CONFIRMED: %s [%d] [%s] [%s]\n"), modifiedURL, statusCode, server, responseTimesSummary)
										}

										// Send request through proxy if configured
										sendProxyRequest(ctx, modifiedURL, *userAgent, *proxy, nil, "", server, responseTimesSummary)

										// Save confirmed SQLI URL if output flag is enabled
										if *output {
											if err := saveConfirmedURL(modifiedURL, *url); err != nil {
												fmt.Printf(Yellow("Warning: Failed to save confirmed URL: %v\n"), err)
											}
										}

										// Launch exploitation tool if on-confirmed flag is set
										if *onConfirmed != "" && *onConfirmed != "none" {
											if err := launchExploitation(modifiedURL, false, *onConfirmed); err != nil {
												fmt.Printf(Yellow("Warning: Failed to launch exploitation: %v\n"), err)
											}
										}

										sqlFoundCount++
										if *stop > 0 && sqlFoundCount >= *stop {
											fmt.Println(Cyan("Stopping further checks for this DOMAIN due to stop flag."))
											stopOnce.Do(cancel)
										}
										return
									}
								} else {
									fmt.Printf(Green("SQLI FP CONFIRMED: %s [%d] [%s] [%s]\n"), modifiedURL, statusCode, server, responseTimesSummary)
								}
							}
						} else {
							fmt.Printf(Green("NOT FOUND: %s [%d] [%s] [%.2f s]\n"), modifiedURL, statusCode, server, responseTime)
						}
					}(payload)
				}
			}
			payloadWg.Wait()
		}
	} else if *list != "" {
		file, err := os.Open(*list)
		if err != nil {
			fmt.Println("Error opening the file:", err)
			return
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		var wg sync.WaitGroup
		sem := make(chan struct{}, *maxParallel)

		for scanner.Scan() {
			url := scanner.Text()
			if strings.Contains(url, "*") {
				sem <- struct{}{}
				wg.Add(1)
				go func(url string) {
					defer func() { <-sem }()

					// Create a new context and cancel function for each URL
					ctx, cancel := context.WithCancel(context.Background())
					stopOnce := &sync.Once{} // Reset stopOnce for each URL
					processURL(ctx, cancel, url, payloads, *responseFlag, *verify, *verifyDelay, *retries, *noColor, *userAgent, *stop, &wg, &mu, stopOnce, *maxConcurrency, *requiredCount, *proxy, *output, *onConfirmed)
				}(url)
			} else {
				fmt.Printf(Cyan("Skipping URL (Not * found): %s\n"), url)
			}
		}
		wg.Wait()

		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading the file:", err)
		}
	} else {
		fmt.Println("Please provide either a URL with -u, a file with -list, or a request file with -r")
	}
}
