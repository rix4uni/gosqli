## gosqli

gosqli is a fast and simple tool for detecting blind SQL injection vulnerabilities. It supports scanning URLs with custom payloads, parallel requests, and response time-based verification.

## Installation
```
go install github.com/rix4uni/gosqli@latest
```

## Download prebuilt binaries
```
wget https://github.com/rix4uni/gosqli/releases/download/v0.0.1/gosqli-linux-amd64-0.0.1.tgz
tar -xvzf gosqli-linux-amd64-0.0.1.tgz
rm -rf gosqli-linux-amd64-0.0.1.tgz
mv gosqli ~/go/bin/gosqli
```
Or download [binary release](https://github.com/rix4uni/gosqli/releases) for your platform.

## Compile from source
```
git clone --depth 1 github.com/rix4uni/gosqli.git
cd gosqli; go install
```

## Usage
```
Usage of gosqli:
  -H string
        Custom User-Agent header for HTTP requests. (default "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36")
  -concurrency int
        Maximum number of Payloads Scan concurrent. (default 20)
  -list string
        File containing list of URLs
  -mrt int
        Match response time with specified response time in seconds. (default 10)
  -nc
        Do not save colored output.
  -parallel int
        Maximum number of URLs Scan Parallely. (default 1)
  -payload string
        File containing payloads
  -requiredCount int
        Number of response times greater than responseFlag required for SQLI CONFIRMED (0 means all).
  -retries int
        Number of retry attempts for failed HTTP requests.
  -silent
        silent mode.
  -stop int
        Stop checking pending HTTP requests after [stop] (0: means check all). (default 1)
  -u string
        URL to fetch
  -verify int
        Number of times to verify "SQLI FOUND". (default 3)
  -verifydelay int
        Delay in milliseconds between verify attempts. (default 12000)
  -version
        Print the version of the tool and exit.
```

## Usage Examples
Single URLs:
```
▶ gosqli -u "http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*" -payload payloads/generic.txt
```

Multiple URLs:
```
▶ cat urls.txt
http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*
http://testphp.vulnweb.com/artists.php?artist=1*

▶ gosqli -list urls.txt -payload payloads/generic.txt
```

Oneliner:
```
▶ echo "testphp.vulnweb.com" | waybackurls | urldedupe -s | pvreplace -silent -payload "*" -fuzzing-part param-value -fuzzing-type replace -fuzzing-mode single | unew -ef -el -t -i -q urls.txt
▶ gosqli -list urls.txt -payload payloads/generic.txt | tee -a gosqli.txt
```