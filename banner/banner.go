package banner

import (
	"fmt"
)

// prints the version message
const version = "v0.0.1"

func PrintVersion() {
	fmt.Printf("Current gosqli version %s\n", version)
}

// Prints the Colorful banner
func PrintBanner() {
	banner := `                               __ _ 
   ____ _ ____   _____ ____ _ / /(_)
  / __  // __ \ / ___// __  // // / 
 / /_/ // /_/ /(__  )/ /_/ // // /  
 \__, / \____//____/ \__, //_//_/   
/____/                 /_/
`
	fmt.Printf("%s\n%40s\n\n", banner, "Current gosqli version "+version)
}
