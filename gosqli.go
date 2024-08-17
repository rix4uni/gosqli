package main

import (
    "bufio"
    "flag"
    "fmt"
    "net/http"
    "os"
    "strings"
    "time"
    "bytes"
    "encoding/json"
    "io/ioutil"
    "gopkg.in/yaml.v2"
    "os/exec"
    "github.com/fatih/color"
    "net/url"
)

const version = "0.0.1"

func printVersion() {
    fmt.Printf("gosqli version %s\n", version)
}

func printBanner() {
    banner := `  ▄████  ▒█████    ██████   █████   ██▓     ██▓
 ██▒ ▀█▒▒██▒  ██▒▒██    ▒ ▒██▓  ██▒▓██▒    ▓██▒
▒██░▄▄▄░▒██░  ██▒░ ▓██▄   ▒██▒  ██░▒██░    ▒██▒
░▓█  ██▓▒██   ██░  ▒   ██▒░██  █▀ ░▒██░    ░██░
░▒▓███▀▒░ ████▓▒░▒██████▒▒░▒███▒█▄ ░██████▒░██░
 ░▒   ▒ ░ ▒░▒░▒░ ▒ ▒▓▒ ▒ ░░░ ▒▒░ ▒ ░ ▒░▓  ░░▓
  ░   ░   ░ ▒ ▒░ ░ ░▒  ░ ░ ░ ▒░  ░ ░ ░ ▒  ░ ▒ ░
░ ░   ░ ░ ░ ░ ▒  ░  ░  ░     ░   ░   ░ ░    ▒ ░
      ░     ░ ░        ░      ░        ░  ░ ░`
    fmt.Printf(Cyan("%s\n%55s\n"), banner, "gosqli version "+version)
}


// Declare package-level color functions
var Red = color.New(color.FgRed).SprintFunc()    // SQLI FOUND, SQLI CONFIRMED
var Green = color.New(color.FgGreen).SprintFunc()    // NOT FOUND, SQLI NOT CONFIRMED
var Yellow = color.New(color.FgYellow).SprintFunc()    // NORMAL REQUEST, RETRYING REQUEST
var Magenta = color.New(color.FgMagenta).SprintFunc()    // sqlFoundCount
var Cyan = color.New(color.FgCyan).SprintFunc()

func fetchURL(urlStr string, userAgent string, retries int, proxy string) (int, string, float64, error) {
    var lastErr error
    var statusCode int
    var server string
    var responseTime float64

    for attempt := 0; attempt <= retries; attempt++ {
        startTime := time.Now()
        req, err := http.NewRequest("GET", urlStr, nil)
        if err != nil {
            lastErr = err
            continue
        }
        req.Header.Set("User-Agent", userAgent)

        // Create the HTTP client with optional proxy
        client := &http.Client{}
        if proxy != "" {
            proxyURL, err := url.Parse(proxy)
            if err != nil {
                return 0, "", 0, fmt.Errorf("invalid proxy URL: %w", err)
            }
            transport := &http.Transport{
                Proxy: http.ProxyURL(proxyURL),
            }
            client = &http.Client{Transport: transport}
        }

        resp, err := client.Do(req)
        if err != nil {
            lastErr = err
            if attempt < retries {
                fmt.Printf(Yellow("RETRYING REQUEST: %s (attempt %d/%d)\n"), urlStr, attempt+1, retries)
                // time.Sleep(2 * time.Second) // Optional: add a delay before retrying
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

func verifyURL(url string, verifyCount int, responseFlag float64, verifyDelay float64, userAgent string, retries int, proxy string) (string, bool, error) {
    var responseTimes []float64
    for i := 0; i < verifyCount; i++ {
        _, _, responseTime, err := fetchURL(url, userAgent, retries, proxy)
        if err != nil {
            return "", false, err
        }
        responseTimes = append(responseTimes, responseTime)
        if responseTime > responseFlag {
            // Continue checking but mark as SQLI FOUND
        }
        time.Sleep(time.Duration(verifyDelay) * time.Second) // Small delay between checks
    }
    isVerified := len(responseTimes) > 0 && responseTimes[len(responseTimes)-1] > responseFlag

    // Create the formatted response times string
    var responseTimesStr []string
    for _, rt := range responseTimes {
        responseTimesStr = append(responseTimesStr, fmt.Sprintf("%.2f s", rt))
    }
    responseTimesSummary := strings.Join(responseTimesStr, ", ")

    return responseTimesSummary, isVerified, nil
}

// Config structure to hold the YAML data
type Config struct {
    Discord struct {
        WebhookURL string `yaml:"webhook_url"`
    } `yaml:"discord"`
}

// Function to load the configuration from a YAML file
func loadConfig(configFile string) (*Config, error) {
    config := &Config{}

    // Read the YAML file
    file, err := ioutil.ReadFile(configFile)
    if err != nil {
        return nil, err
    }

    // Unmarshal the YAML data into the Config struct
    err = yaml.Unmarshal(file, config)
    if err != nil {
        return nil, err
    }

    return config, nil
}

// Function to send a message to Discord
func discord(webhookURL, messageContent string) {
    // Create a map to hold the JSON payload
    payload := map[string]string{
        "content": messageContent,
    }

    // Marshal the payload to JSON
    payloadBytes, err := json.Marshal(payload)
    if err != nil {
        fmt.Println("Error marshaling payload:", err)
        return
    }

    // Create a new POST request with the payload
    req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(payloadBytes))
    if err != nil {
        fmt.Println("Error creating request:", err)
        return
    }

    // Set the Content-Type header to application/json
    req.Header.Set("Content-Type", "application/json")

    // Send the request
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        fmt.Println("Error sending request:", err)
        return
    }
    defer resp.Body.Close()

    // Check if the request was successful
    if resp.StatusCode == http.StatusNoContent {
        fmt.Println(Cyan("\"SQLI CONFIRMED\" Message sent to Discord successfully!"))
    } else {
        fmt.Printf("Failed to send message. Status code: %d\n", resp.StatusCode)
    }
}

// Function to generate a unique tmux session name
func generateUniqueSessionName(baseName string) string {
    sessionName := baseName
    sessionNumber := 0

    for {
        // Check if the session already exists
        cmd := exec.Command("tmux", "has-session", "-t", sessionName)
        if err := cmd.Run(); err != nil {
            // If the session doesn't exist, break the loop
            break
        }
        // If it exists, increment the session number and try again
        sessionNumber++
        sessionName = fmt.Sprintf("%s%d", baseName, sessionNumber)
    }

    return sessionName
}

func extractDomain(u string) string {
    parsedURL, err := url.Parse(u)
    if err != nil {
        return u // If parsing fails, return the original URL
    }
    return parsedURL.Hostname()
}

func main() {
    urlStr := flag.String("u", "", "URL to fetch")
    list := flag.String("list", "", "File containing list of URLs")
    payloadFile := flag.String("payload", "", "File containing payloads")
    responseFlag := flag.Int("mrt", 10, "Match response time with specified response time in seconds.")
    verify := flag.Int("verify", 2, "Number of times to verify \"SQLI FOUND\".")
    verifyDelay := flag.Int("verifydelay", 3, "Delay in seconds between verify attempts.")
    retries := flag.Int("retries", 1, "Number of retry attempts for failed HTTP requests.")
    outputFile := flag.String("o", "", "File to save the output.")
    appendOutput := flag.String("ao", "", "File to append the output instead of overwriting.")
    silent := flag.Bool("silent", false, "silent mode.")
    noColor := flag.Bool("nc", false, "Do not Save colored output.")
    stop := flag.Int("stop", 1, "Stop checking pending HTTP requests after [stop] (0: means check all).")
    userAgent := flag.String("H", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36", "Custom User-Agent header for HTTP requests.")
    version := flag.Bool("version", false, "Print the version of the tool and exit.")
    verbose := flag.Bool("verbose", false, "Enable verbose output for debugging purposes.")
    icoutput := flag.Bool("icoutput", false, "File to save the integratecmd output.")
    sendToDiscord  := flag.Bool("discord", false, "Send \"SQLI CONFIRMED\" to Discord Webhook URL.")
    configPath := flag.String("config", "", "path to the config.yaml file")
    maxsca := flag.Int("maxsca", 20, "Maximum Number of \"403\" Status Code Allowed before skipping all URLs from that domain.")
    integratecmd := flag.String("integratecmd", "", "Send \"SQLI CONFIRMED\" to sqlmap/ghauri command via tmux")
    proxy := flag.String("proxy", "", "HTTP proxy to use for requests (e.g., http://127.0.0.1:8080)") // Proxy flag
    flag.Parse()

    // Print version and exit if -version flag is provided
    if *version {
        printVersion()
        return
    }

    // Don't Print banner if -silnet flag is provided
    if !*silent {
        printBanner()
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

    // Create or open output file if specified
    var output *os.File
    var err error // Declare err here
    if *outputFile != "" {
        output, err = os.Create(*outputFile)
        if err != nil {
            fmt.Println("Error creating output file:", err)
            return
        }
        defer output.Close()
    } else if *appendOutput != "" {
        output, err = os.OpenFile(*appendOutput, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
        if err != nil {
            fmt.Println("Error opening output file for appending:", err)
            return
        }
        defer output.Close()
    }

    // Initialize a counter for SQLI FOUND results
    sqlFoundCount := 0

    // Check if both -config and -discord flags are used together
    if (*sendToDiscord && *configPath == "") || (!*sendToDiscord && *configPath != "") {
        fmt.Println("Error: Both -config and -discord must be provided together.")
        os.Exit(1)
    }

    var config *Config

    // Only load the configuration if -config is provided
    if *sendToDiscord && *configPath != "" {
        config, err = loadConfig(*configPath)
        if err != nil {
            fmt.Println("Error loading config:", err)
            os.Exit(1)
        }
    }

    // Initialize a counter for 403 status codes
    forbiddenCount := 0

    if *urlStr != "" {
        if strings.Contains(*urlStr, "*") {
            fmt.Printf(Yellow("ORIGINAL URL: %s\n"), *urlStr)
            noStarURL := strings.Replace(*urlStr, "*", "", -1)
            statusCode, server, responseTime, err := fetchURL(noStarURL, *userAgent, *retries, *proxy)
            if err != nil {
                fmt.Println("Error fetching the URL:", err)
                return
            }
            fmt.Printf(Yellow("NORMAL REQUEST: %s [%d] [%s] [%.2f s]\n"), noStarURL, statusCode, server, responseTime)
            
            nStars := strings.Count(*urlStr, "*")
            for _, payload := range payloads {
                for i := 0; i < nStars; i++ {
                    modifiedURL := *urlStr
                    starCount := 0

                    // Replace the ith '*' with the payload
                    for j := 0; j < len(modifiedURL); j++ {
                        if modifiedURL[j] == '*' {
                            if starCount == i {
                                modifiedURL = modifiedURL[:j] + payload + modifiedURL[j+1:]

                                break
                            }
                            starCount++
                        }
                    }
                    noModifiedStarURL := strings.Replace(modifiedURL, "*", "", -1)
                    statusCode, server, responseTime, err := fetchURL(noModifiedStarURL, *userAgent, *retries, *proxy)
                    if err != nil {
                        fmt.Println("Error fetching the URL:", err)
                        continue
                    }

                    // Adding output in a empty variable
                    outputStr := ""
                    if responseTime > float64(*responseFlag) {
                        if *noColor {
                            outputStr = fmt.Sprintf("SQLI FOUND: %s [%d] [%s] [%.2f s]\n", noModifiedStarURL, statusCode, server, responseTime)
                        } else {
                            outputStr = fmt.Sprintf(Red("SQLI FOUND: %s [%d] [%s] [%.2f s]\n"), noModifiedStarURL, statusCode, server, responseTime)
                        }
                        fmt.Print(outputStr) // Print to the terminal
                        if output != nil {
                            output.WriteString(outputStr) // Save to the output file
                        }

                        if *verify > 1 {
                            responseTimesSummary, isVerified, err := verifyURL(noModifiedStarURL, *verify, float64(*responseFlag), float64(*verifyDelay), *userAgent, *retries, *proxy)
                            if err != nil {
                                fmt.Println("Error verifying the URL:", err)
                                continue
                            }
                            if isVerified {
                                if *noColor {
                                outputStr = fmt.Sprintf("SQLI CONFIRMED: %s [%d] [%s] [%s]\n", noModifiedStarURL, statusCode, server, responseTimesSummary)
                                } else {
                                    outputStr = fmt.Sprintf(Red("SQLI CONFIRMED: %s [%d] [%s] [%s]\n"), noModifiedStarURL, statusCode, server, responseTimesSummary)
                                }

                                fmt.Print(outputStr)
                                if output != nil {
                                    output.WriteString(outputStr)
                                }

                                // Call the discord function with the loaded webhookURL and messageContent
                                if *sendToDiscord && config != nil {
                                    // The message content
                                    messageContent := fmt.Sprintf("```SQLI CONFIRMED: %s [%d] [%s] [%s]```\n", noModifiedStarURL, statusCode, server, responseTimesSummary)
                                    discord(config.Discord.WebhookURL, messageContent)
                                }

                                sqlFoundCount++ // Increment the counter
                                if *stop > 0 && sqlFoundCount >= *stop {
                                    fmt.Printf(Cyan("Stopping further checks for this URL (%s) due to -stop flag.\n"), *urlStr)

                                    if *integratecmd != "" {
                                        if *icoutput {
                                            // Generate a unique session name
                                            sessionName := generateUniqueSessionName("integratecmdSession")

                                            // Prepare the echo command
                                            echoCmdStr := fmt.Sprintf("echo Running ghauri: %s | tee -a %s.log", fmt.Sprintf("\\\"%s\\\"", noModifiedStarURL), sessionName)

                                            // Prepare the ghauri command with the URL in double quotes and run it via tmux
                                            ghauriCmdStr := strings.Replace(*integratecmd, "{urlStr}", fmt.Sprintf("\\\"%s\\\"", noModifiedStarURL), -1)

                                            // Prepare the ghauri finished command
                                            ghauriFinished := fmt.Sprintf("echo Finished ghauri: %s | tee -a %s.log", fmt.Sprintf("\\\"%s\\\"", noModifiedStarURL), sessionName)

                                            // Combine the ghauri command with unbuffer and save them output via tee command, using sessionName.log
                                            combinedCmdStr := fmt.Sprintf("%s && unbuffer %s | tee -a %s.log && %s", echoCmdStr, ghauriCmdStr, sessionName, ghauriFinished)

                                            // Wrap the ghauri command in a tmux command with the unique session name
                                            tmuxCmdStr := fmt.Sprintf("tmux new-session -d -s %s 'bash -c \"%s\"; bash'", sessionName, combinedCmdStr)

                                            runCmdStr := fmt.Sprintf("tmux new-session -d -s %s \"%s\"", sessionName, ghauriCmdStr)
                                            fmt.Printf(Cyan("Running: %s\n"), runCmdStr)
                                            fmt.Printf(Cyan("Attach tmux session: tmux a -t %s\n"), sessionName)

                                            // Run the tmux command with bash
                                            cmd := exec.Command("bash", "-c", tmuxCmdStr)
                                            cmd.Stdout = os.Stdout
                                            cmd.Stderr = os.Stderr
                                            if err := cmd.Run(); err != nil {
                                                fmt.Printf("Error running ghauri command in tmux: %s\n", err)
                                            }
                                        } else {
                                            // Generate a unique session name
                                            sessionName := generateUniqueSessionName("integratecmdSession")

                                            // Prepare the echo command
                                            echoCmdStr := fmt.Sprintf("echo Running ghauri: %s", fmt.Sprintf("\\\"%s\\\"", noModifiedStarURL))

                                            // Prepare the ghauri command with the URL in double quotes and run it via tmux
                                            ghauriCmdStr := strings.Replace(*integratecmd, "{urlStr}", fmt.Sprintf("\\\"%s\\\"", noModifiedStarURL), -1)

                                            // Prepare the ghauri finished command
                                            ghauriFinished := fmt.Sprintf("echo Finished ghauri: %s", fmt.Sprintf("\\\"%s\\\"", noModifiedStarURL))

                                            // Combine all commands
                                            combinedCmdStr := fmt.Sprintf("%s && %s && %s", echoCmdStr, ghauriCmdStr, ghauriFinished)

                                            // Wrap the ghauri command in a tmux command with the unique session name
                                            tmuxCmdStr := fmt.Sprintf("tmux new-session -d -s %s 'bash -c \"%s\"; bash'", sessionName, combinedCmdStr)

                                            runCmdStr := fmt.Sprintf("tmux new-session -d -s %s \"%s\"", sessionName, ghauriCmdStr)
                                            fmt.Printf(Cyan("Running: %s\n"), runCmdStr)
                                            fmt.Printf(Cyan("Attach tmux session: tmux a -t %s\n"), sessionName)

                                            // Run the tmux command with bash
                                            cmd := exec.Command("bash", "-c", tmuxCmdStr)
                                            cmd.Stdout = os.Stdout
                                            cmd.Stderr = os.Stderr
                                            if err := cmd.Run(); err != nil {
                                                fmt.Printf("Error running ghauri command in tmux: %s\n", err)
                                            }
                                        }
                                    }
                                    break // Exit the payload loop for the current URL
                                }
                            } else {
                                fmt.Printf(Green("SQLI FP CONFIRMED: %s [%d] [%s] [%s]\n"), noModifiedStarURL, statusCode, server, responseTimesSummary)
                            }
                        }
                } else {
                    fmt.Printf(Green("NOT FOUND: %s [%d] [%s] [%.2f s]\n"), noModifiedStarURL, statusCode, server, responseTime)
                }
                fmt.Print(outputStr)
                if output != nil {
                    output.WriteString(outputStr)
                }
            }
            if *stop > 0 && sqlFoundCount >= *stop {
                break // Break out of the main URL loop if stop condition is met
            }
        }
    }
    } else if *list != "" {
        file, err := os.Open(*list)
        if err != nil {
            fmt.Println("Error opening the file:", err)
            return
        }
        defer file.Close()

        scanner := bufio.NewScanner(file)
        for scanner.Scan() {
            urlStr := scanner.Text()
            if strings.Contains(urlStr, "*") {
                originalURL := urlStr
                fmt.Printf(Yellow("ORIGINAL URL: %s\n"), originalURL)
                noStarURL := strings.Replace(urlStr, "*", "", -1)
                statusCode, server, responseTime, err := fetchURL(noStarURL, *userAgent, *retries, *proxy)
                if err != nil {
                    fmt.Println("Error fetching the URL:", err)
                    continue
                }
                fmt.Printf(Yellow("NORMAL REQUEST: %s [%d] [%s] [%.2f s]\n"), noStarURL, statusCode, server, responseTime)

                starIndexes := []int{}
                for i := 0; i < len(urlStr); i++ {
                    if urlStr[i] == '*' {
                        starIndexes = append(starIndexes, i)
                    }
                }

                stopProcessing := false

                for _, payload := range payloads {
                    for _, index := range starIndexes {
                        if stopProcessing {
                            break
                        }

                        modifiedURL := urlStr[:index] + payload + urlStr[index+1:]

                        noModifiedStarURL := strings.Replace(modifiedURL, "*", "", -1)
                        statusCode, server, responseTime, err := fetchURL(noModifiedStarURL, *userAgent, *retries, *proxy)
                        if err != nil {
                            fmt.Println("Error fetching the URL:", err)
                            continue
                        }

                        // Check if status code is 403
                        if statusCode == 403 {
                            forbiddenCount++
                            if forbiddenCount > *maxsca {
                                domain := extractDomain(urlStr)
                                fmt.Printf(Magenta("Skipping remaining URLs: for this DOMAIN (%s) due to 403 response limit reached -maxsca.\n"), domain)
                                break
                            }
                        }

                        outputStr := ""
                        if responseTime > float64(*responseFlag) {
                            if *noColor {
                                outputStr = fmt.Sprintf("SQLI FOUND: %s [%d] [%s] [%.2f s]\n", noModifiedStarURL, statusCode, server, responseTime)
                            } else {
                                outputStr = fmt.Sprintf(Red("SQLI FOUND: %s [%d] [%s] [%.2f s]\n"), noModifiedStarURL, statusCode, server, responseTime)
                            }
                            fmt.Print(outputStr)
                            if output != nil {
                                output.WriteString(outputStr)
                            }

                            if *verify > 1 {
                                responseTimesSummary, isVerified, err := verifyURL(noModifiedStarURL, *verify, float64(*responseFlag), float64(*verifyDelay), *userAgent, *retries, *proxy)
                                if err != nil {
                                    fmt.Println("Error verifying the URL:", err)
                                    continue
                                }
                                if isVerified {
                                    if *noColor {
                                        outputStr = fmt.Sprintf("SQLI CONFIRMED: %s [%d] [%s] [%s]\n", noModifiedStarURL, statusCode, server, responseTimesSummary)
                                    } else {
                                        outputStr = fmt.Sprintf(Red("SQLI CONFIRMED: %s [%d] [%s] [%s]\n"), noModifiedStarURL, statusCode, server, responseTimesSummary)
                                    }

                                    fmt.Print(outputStr)
                                    if output != nil {
                                        output.WriteString(outputStr)
                                    }

                                    if *sendToDiscord && config != nil {
                                        messageContent := fmt.Sprintf("```SQLI CONFIRMED: %s [%d] [%s] [%s]```\n", noModifiedStarURL, statusCode, server, responseTimesSummary)
                                        discord(config.Discord.WebhookURL, messageContent)
                                    }

                                    sqlFoundCount++
                                    if *stop > 0 && sqlFoundCount >= *stop {
                                        fmt.Printf(Cyan("Stopping further checks for this URL (%s) due to -stop flag.\n"), urlStr)

                                        if *integratecmd != "" {

                                            if *icoutput {
                                                // Generate a unique session name
                                                sessionName := generateUniqueSessionName("integratecmdSession")

                                                // Prepare the echo command
                                                echoCmdStr := fmt.Sprintf("echo Running ghauri: %s | tee -a %s.log", fmt.Sprintf("\\\"%s\\\"", noModifiedStarURL), sessionName)

                                                // Prepare the ghauri command with the URL in double quotes and run it via tmux
                                                ghauriCmdStr := strings.Replace(*integratecmd, "{urlStr}", fmt.Sprintf("\\\"%s\\\"", noModifiedStarURL), -1)

                                                // Prepare the ghauri finished command
                                                ghauriFinished := fmt.Sprintf("echo Finished ghauri: %s | tee -a %s.log", fmt.Sprintf("\\\"%s\\\"", noModifiedStarURL), sessionName)

                                                // Combine the ghauri command with unbuffer and save them output via tee command, using sessionName.log
                                                combinedCmdStr := fmt.Sprintf("%s && unbuffer %s | tee -a %s.log && %s", echoCmdStr, ghauriCmdStr, sessionName, ghauriFinished)

                                                // Wrap the ghauri command in a tmux command with the unique session name
                                                tmuxCmdStr := fmt.Sprintf("tmux new-session -d -s %s 'bash -c \"%s\"; bash'", sessionName, combinedCmdStr)

                                                runCmdStr := fmt.Sprintf("tmux new-session -d -s %s \"%s\"", sessionName, ghauriCmdStr)
                                                fmt.Printf(Cyan("Running: %s\n"), runCmdStr)
                                                fmt.Printf(Cyan("Attach tmux session: tmux a -t %s\n"), sessionName)

                                                // Run the tmux command with bash
                                                cmd := exec.Command("bash", "-c", tmuxCmdStr)
                                                cmd.Stdout = os.Stdout
                                                cmd.Stderr = os.Stderr
                                                if err := cmd.Run(); err != nil {
                                                    fmt.Printf("Error running ghauri command in tmux: %s\n", err)
                                                }
                                            } else {
                                                // Generate a unique session name
                                                sessionName := generateUniqueSessionName("integratecmdSession")

                                                // Prepare the echo command
                                                echoCmdStr := fmt.Sprintf("echo Running ghauri: %s", fmt.Sprintf("\\\"%s\\\"", noModifiedStarURL))

                                                // Prepare the ghauri command with the URL in double quotes and run it via tmux
                                                ghauriCmdStr := strings.Replace(*integratecmd, "{urlStr}", fmt.Sprintf("\\\"%s\\\"", noModifiedStarURL), -1)

                                                // Prepare the ghauri finished command
                                                ghauriFinished := fmt.Sprintf("echo Finished ghauri: %s", fmt.Sprintf("\\\"%s\\\"", noModifiedStarURL))

                                                // Combine both commands
                                                combinedCmdStr := fmt.Sprintf("%s && %s && %s", echoCmdStr, ghauriCmdStr, ghauriFinished)

                                                // Wrap the ghauri command in a tmux command with the unique session name
                                                tmuxCmdStr := fmt.Sprintf("tmux new-session -d -s %s 'bash -c \"%s\"; bash'", sessionName, combinedCmdStr)

                                                runCmdStr := fmt.Sprintf("tmux new-session -d -s %s \"%s\"", sessionName, ghauriCmdStr)
                                                fmt.Printf(Cyan("Running: %s\n"), runCmdStr)
                                                fmt.Printf(Cyan("Attach tmux session: tmux a -t %s\n"), sessionName)

                                                // Run the tmux command with bash
                                                cmd := exec.Command("bash", "-c", tmuxCmdStr)
                                                cmd.Stdout = os.Stdout
                                                cmd.Stderr = os.Stderr
                                                if err := cmd.Run(); err != nil {
                                                    fmt.Printf("Error running ghauri command in tmux: %s\n", err)
                                                }
                                            }
                                        }

                                        stopProcessing = true
                                        break
                                    }
                                } else {
                                    fmt.Printf(Green("SQLI FP CONFIRMED: %s [%d] [%s] [%s]\n"), noModifiedStarURL, statusCode, server, responseTimesSummary)
                                }
                            }
                        } else {
                            fmt.Printf(Green("NOT FOUND: %s [%d] [%s] [%.2f s]\n"), noModifiedStarURL, statusCode, server, responseTime)
                        }
                    }

                    if stopProcessing {
                        break
                    }
                }
            } else {
                if *verbose {
                    fmt.Printf(Cyan("Skipping URL (Not * found): %s\n"), urlStr)
                }
            }
        }

        if err := scanner.Err(); err != nil {
            fmt.Println("Error reading the file:", err)
        }

    } else {
        fmt.Println("Please provide either a URL with -u or a file with -list")
    }
}


// Todo List
// proxy := flag.String("proxy", "", "Proxy server for HTTP requests to send Burpsuite. (e.g., http://127.0.0.1:8080)")


// go run gosqli.go -list urls.txt -payload payloads/generic.txt -o ot.txt
// go run gosqli.go -u "http://testphp.vulnweb.com/artists.php?artist=1*" -payload payloads/generic.txt -o ot.txt

// go run gosqli.go -list urls.txt -payload payloads/generic.txt -o ot.txt -config ~/.config/gosqli/config.yaml -discord -integratecmd "ghauri -u {urlStr} --level 3 --dbs --time-sec 12 --batch --flush-session"
