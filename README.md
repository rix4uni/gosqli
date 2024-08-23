# gosqli

# Usage
```console
Usage of gosqli:
  -H string
    	Custom User-Agent header for HTTP requests. (default "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36")
  -ao string
    	File to append the output instead of overwriting.
  -config string
    	path to the config.yaml file
  -discord
    	Send "SQLI CONFIRMED" to Discord Webhook URL.
  -icoutput
    	File to save the integratecmd output.
  -integratecmd string
    	Send "SQLI CONFIRMED" to sqlmap/ghauri command via tmux
  -list string
    	File containing list of URLs
  -maxsca int
    	Maximum Number of "403" Status Code Allowed before skipping all URLs from that domain. (default 20)
  -mrt int
    	Match response time with specified response time in seconds. (default 10)
  -nc
    	Do not Save colored output.
  -o string
    	File to save the output.
  -payload string
    	File containing payloads
  -proxy string
    	HTTP proxy to use for requests (e.g., http://127.0.0.1:8080)
  -requiredCount int
    	Number of response times greater than responseFlag required for SQLI CONFIRMED (0 means all). (default 2)
  -retries int
    	Number of retry attempts for failed HTTP requests.
  -silent
    	silent mode.
  -stop int
    	Stop checking pending HTTP requests after [stop] (0: means check all). (default 1)
  -u string
    	URL to fetch
  -verbose
    	Enable verbose output for debugging purposes.
  -verify int
    	Number of times to verify "SQLI FOUND". (default 3)
  -verifydelay int
    	Delay in seconds between verify attempts. (default 3)
  -version
    	Print the version of the tool and exit.
```

# Usage Examples
```console
go run gosqli.go -list urls.txt -payload payloads/generic.txt -o ot.txt -icoutput -config config.yaml -discord -integratecmd "ghauri -u {urlStr} --level 3 --dbs --time-sec 12 --batch --flush-session"
```

# flags impimantation explanation
```
```

## **Legal disclaimer**
```
Usage of gosqli for attacking targets without prior mutual consent is illegal.
It is the end user's responsibility to obey all applicable local,state and federal laws. 
Developer assume no liability and is not responsible for any misuse or damage caused by this program.
```

## **TODO**
-p flag to scan urls parallely with -list flag
-c flag to scan payload urls parallely with -u and -list flag
