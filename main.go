package main

import (
    "fmt"
    "strings"
)

func main() {
    url := "http://192.168.1.2/labs/?name=testname*&email=testemail%40gmail.com*&comment=testcomment*&submit="
    newURL := strings.Replace(url, "*", "", -1)
    fmt.Println(newURL)
}
