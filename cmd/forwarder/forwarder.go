package main

import ( "encoding/json" "fmt" "log" "net/http" "os" "strings" )

type IncomingPayload struct { Title   string   json:"title" Content string   json:"content" Tags    []string json:"tags" }

func main() { port := os.Getenv("FORWARDER_PORT") if port == "" { port = "9020" }

http.HandleFunc("/forward", handleForward)
log.Println("Sankarea forwarder listening on port", port)
log.Fatal(http.ListenAndServe(":"+port, nil))

}

func handleForward(w http.ResponseWriter, r *http.Request) { if r.Method != http.MethodPost { http.Error(w, "Invalid method", http.StatusMethodNotAllowed) return }

token := r.Header.Get("Authorization")
if token != os.Getenv("FORWARDER_TOKEN") {
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	return
}

var payload IncomingPayload
if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
	http.Error(w, "Invalid JSON", http.StatusBadRequest)
	return
}

discord := os.Getenv("DISCORD_WEBHOOKS")
if discord == "" {
	http.Error(w, "Missing webhook", http.StatusInternalServerError)
	return
}

tags := ""
if len(payload.Tags) > 0 {
	tags = "\n**Tags:** " + strings.Join(payload.Tags, ", ")
}

msg := map[string]interface{}{
	"embeds": []map[string]interface{}{
		{
			"title": payload.Title,
			"description": payload.Content + tags,
			"color": 16753920, // orange
		},
	},
}
body, _ := json.Marshal(msg)

for _, hook := range strings.Split(discord, ",") {
	resp, err := http.Post(hook, "application/json", strings.NewReader(string(body)))
	if err != nil {
		log.Println("Failed to forward:", err)
		continue
	}
	defer resp.Body.Close()
	fmt.Fprintf(w, "Forwarded to %s\n", hook)
}

}

