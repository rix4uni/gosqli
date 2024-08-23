package main

import (
    "bufio"
    "context"
    "flag"
    "fmt"
    "net/http"
    "os"
    "strings"
    "sync"
    "time"
    "github.com/fatih/color"
)

// Declare package-level color functions
var Red = color.New(color.FgRed).SprintFunc()
var Green = color.New(color.FgGreen).SprintFunc()
var Yellow = color.New(color.FgYellow).SprintFunc()
var Magenta = color.New(color.FgMagenta).SprintFunc()
var Cyan = color.New(color.FgCyan).SprintFunc()

func fetchURL(ctx context.Context, url string, userAgent string, retries int) (int, string, float64, error) {
    var lastErr error
    var statusCode int
    var server string
    var responseTime float64

    for attempt := 0; attempt <= retries; attempt++ {
        startTime := time.Now()
        req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
        if err != nil {
            lastErr = err
            continue
        }
        req.Header.Set("User-Agent", userAgent)

        client := &http.Client{}
        resp, err := client.Do(req)
        if err != nil {
            lastErr = err
            if attempt < retries {
                fmt.Printf(Yellow("RETRYING REQUEST: %s (attempt %d/%d)\n"), url, attempt+1, retries)
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

func verifyURL(ctx context.Context, url string, verifyCount int, responseFlag float64, verifyDelay float64, userAgent string, retries int, requiredCount int) (string, bool, error) {
    var responseTimes []float64
    for i := 0; i < verifyCount; i++ {
        _, _, responseTime, err := fetchURL(ctx, url, userAgent, retries)
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

func processURL(ctx context.Context, cancel context.CancelFunc, url string, payloads []string, responseFlag, verify, verifyDelay, retries int, noColor bool, userAgent string, stop int, wg *sync.WaitGroup, mu *sync.Mutex, stopOnce *sync.Once, maxConcurrency int, requiredCount int) {
    defer wg.Done()

    sqlFoundCount := 0 // Reset for each URL

    statusCode, server, responseTime, err := fetchURL(ctx, url, userAgent, retries)
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

                modifiedURL := strings.Replace(url, "*", payload, -1)
                statusCode, server, responseTime, err := fetchURL(ctx, modifiedURL, userAgent, retries)
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
                        responseTimesSummary, isVerified, err := verifyURL(ctx, modifiedURL, verify, float64(responseFlag), float64(verifyDelay), userAgent, retries, requiredCount)
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

func main() {
    url := flag.String("u", "", "URL to fetch")
    list := flag.String("list", "", "File containing list of URLs")
    payloadFile := flag.String("payload", "", "File containing payloads")
    responseFlag := flag.Int("mrt", 10, "Match response with specified response time in seconds.")
    verify := flag.Int("verify", 3, "Number of times to verify \"SQLI FOUND\".")
    requiredCount := flag.Int("requiredCount", 2, "Number of response times greater than responseFlag required for SQLI CONFIRMED (0 means all).")
    verifyDelay := flag.Int("verifydelay", 3000, "Delay in milliseconds between verify attempts.")
    retries := flag.Int("retries", 1, "Number of retry attempts for failed HTTP requests.")
    noColor := flag.Bool("nc", false, "Do not save colored output.")
    stop := flag.Int("stop", 1, "Stop checking pending HTTP requests after [stop] (0: means check all).")
    userAgent := flag.String("H", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36", "Custom User-Agent header for HTTP requests.")
    maxParallel := flag.Int("parallel", 50, "Maximum number of URLs Scan Parallely.")
    maxConcurrency := flag.Int("concurrency", 50, "Maximum number of Payloads Scan concurrent.")
    flag.Parse()

    // Display flag values at the start of the program
    fmt.Println("-------------------------------------------")
    fmt.Printf(" :: responseFlag    : %d\n", *responseFlag)
    fmt.Printf(" :: verify          : %d\n", *verify)
    fmt.Printf(" :: requiredCount   : %d\n", *requiredCount)
    fmt.Printf(" :: verifyDelay     : %d\n", *verifyDelay)
    fmt.Printf(" :: retries         : %d\n", *retries)
    fmt.Printf(" :: stop            : %d\n", *stop)
    fmt.Printf(" :: maxParallel     : %d\n", *maxParallel)
    fmt.Printf(" :: maxConcurrency  : %d\n", *maxConcurrency)
    fmt.Println("-------------------------------------------")

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
    }

    var mu sync.Mutex
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sqlFoundCount := 0
    stopOnce := &sync.Once{}

    if *url != "" {
        if strings.Contains(*url, "*") {
            statusCode, server, responseTime, err := fetchURL(ctx, *url, *userAgent, *retries)
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

                        modifiedURL := strings.Replace(*url, "*", payload, -1)
                        statusCode, server, responseTime, err := fetchURL(ctx, modifiedURL, *userAgent, *retries)
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
                                responseTimesSummary, isVerified, err := verifyURL(ctx, modifiedURL, *verify, float64(*responseFlag), float64(*verifyDelay), *userAgent, *retries, *requiredCount)
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
                    processURL(ctx, cancel, url, payloads, *responseFlag, *verify, *verifyDelay, *retries, *noColor, *userAgent, *stop, &wg, &mu, stopOnce, *maxConcurrency, *requiredCount)
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
        fmt.Println("Please provide either a URL with -u or a file with -list")
    }
}
