package main

import ( "bytes" "encoding/json" "flag" "fmt" "io/ioutil" "net/http" "os" )

func main() { command := flag.String("cmd", "listtags", "Command: addtag, removetag, listtags") value := flag.String("val", "", "Tag value (if required)") host := flag.String("host", "http://localhost:9010", "Command server base URL") flag.Parse()

cmd := map[string]string{
	"command": *command,
	"value": *value,
}
body, _ := json.Marshal(cmd)

req, _ := http.NewRequest("POST", *host+"/command", bytes.NewBuffer(body))
req.Header.Set("Authorization", os.Getenv("COMMAND_TOKEN"))
req.Header.Set("Content-Type", "application/json")
resp, err := http.DefaultClient.Do(req)
if err != nil {
	fmt.Println("Request failed:", err)
	return
}
defer resp.Body.Close()
data, _ := ioutil.ReadAll(resp.Body)
fmt.Printf("Response [%d]: %s\n", resp.StatusCode, data)

}


